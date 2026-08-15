#!/usr/bin/env python3
"""Run an ACP boot, resume, and provider-recovery smoke against a carrier."""

from __future__ import annotations

import argparse
import http.server
import json
import os
from pathlib import Path
import subprocess
import threading
import time


class MockDeepSeek(http.server.BaseHTTPRequestHandler):
    requests: list[dict] = []

    def do_POST(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler contract
        length = int(self.headers.get("content-length", "0"))
        body = self.rfile.read(length)
        request = json.loads(body)
        self.requests.append(request)
        if "trigger-provider-error" in json.dumps(request):
            encoded = json.dumps({"error": {"message": "simulated provider outage"}}).encode()
            self.send_response(503)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)
            return
        chunks = [
            {"choices": [{"delta": {"role": "assistant", "content": "LINUX_CARRIER_OK"}, "finish_reason": None}]},
            {
                "choices": [{"delta": {}, "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
            },
        ]
        payload = "".join(f"data: {json.dumps(chunk)}\n\n" for chunk in chunks) + "data: [DONE]\n\n"
        encoded = payload.encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, *_args: object) -> None:
        return


def send(process: subprocess.Popen[str], frame: dict) -> None:
    assert process.stdin is not None
    process.stdin.write(json.dumps(frame) + "\n")
    process.stdin.flush()


def read_response(process: subprocess.Popen[str], request_id: int) -> tuple[list[str], dict]:
    assert process.stdout is not None
    lines: list[str] = []
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        line = process.stdout.readline()
        if not line:
            break
        lines.append(line.rstrip())
        try:
            frame = json.loads(line)
        except json.JSONDecodeError:
            continue
        if str(frame.get("id")) == str(request_id):
            return lines, frame
    raise RuntimeError(f"timed out waiting for ACP response {request_id}: {lines}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--carrier", required=True, type=Path)
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--workspace", required=True, type=Path)
    args = parser.parse_args()
    args.carrier = args.carrier.resolve()
    args.config = args.config.resolve()
    args.workspace = args.workspace.resolve()
    args.workspace.mkdir(parents=True, exist_ok=True)

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), MockDeepSeek)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    env = os.environ.copy()
    env.update(
        {
            "DEEPSEEK_API_KEY": "carrier-smoke-key",
            "DEEPSEEK_BASE_URL": f"http://127.0.0.1:{server.server_port}",
            "DSH_NETWORK_MODE": "deny",
        }
    )
    def launch() -> subprocess.Popen[str]:
        return subprocess.Popen(
            [str(args.carrier), "--config", str(args.config)],
            cwd=args.workspace,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )

    process = launch()
    session_id = ""
    try:
        send(process, {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": 1, "clientCapabilities": {}}})
        _, initialized = read_response(process, 1)
        if "result" not in initialized:
            raise RuntimeError(f"ACP initialize failed: {initialized}")

        send(process, {"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": {"cwd": str(args.workspace), "mcpServers": []}})
        _, created = read_response(process, 2)
        session_id = created.get("result", {}).get("sessionId", "")
        if not session_id:
            raise RuntimeError(f"ACP session/new returned no session: {created}")

        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "session/prompt",
                "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": "Reply with the smoke marker only."}]},
            },
        )
        lines, prompted = read_response(process, 3)
        output = "\n".join(lines)
        if "result" not in prompted or "LINUX_CARRIER_OK" not in output:
            raise RuntimeError(f"ACP prompt failed: {prompted}; output={output}")
        if len(MockDeepSeek.requests) != 1:
            raise RuntimeError(f"expected one mock provider request, got {len(MockDeepSeek.requests)}")

        send(process, {"jsonrpc": "2.0", "id": 4, "method": "session/close", "params": {"sessionId": session_id}})
        _, closed = read_response(process, 4)
        if "result" not in closed:
            raise RuntimeError(f"ACP session/close failed: {closed}")
        assert process.stdin is not None
        process.stdin.close()
        process.wait(timeout=10)

        process = launch()
        send(process, {"jsonrpc": "2.0", "id": 5, "method": "initialize", "params": {"protocolVersion": 1, "clientCapabilities": {}}})
        _, resumed_initialized = read_response(process, 5)
        if "result" not in resumed_initialized:
            raise RuntimeError(f"ACP resume initialize failed: {resumed_initialized}")
        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 6,
                "method": "session/resume",
                "params": {"sessionId": session_id, "cwd": str(args.workspace), "mcpServers": []},
            },
        )
        _, resumed = read_response(process, 6)
        if "result" not in resumed:
            raise RuntimeError(f"ACP session/resume failed: {resumed}")
        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 7,
                "method": "session/prompt",
                "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": "Reply with the resume marker only."}]},
            },
        )
        resumed_lines, resumed_prompt = read_response(process, 7)
        resumed_output = "\n".join(resumed_lines)
        if "result" not in resumed_prompt or "LINUX_CARRIER_OK" not in resumed_output:
            raise RuntimeError(f"ACP resumed prompt failed: {resumed_prompt}; output={resumed_output}")
        if len(MockDeepSeek.requests) != 2:
            raise RuntimeError(f"expected two mock provider requests after resume, got {len(MockDeepSeek.requests)}")

        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 8,
                "method": "session/prompt",
                "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": "trigger-provider-error"}]},
            },
        )
        _, failed_prompt = read_response(process, 8)
        if "error" not in failed_prompt:
            raise RuntimeError(f"provider failure did not surface as ACP error: {failed_prompt}")

        send(process, {"jsonrpc": "2.0", "id": 9, "method": "session/close", "params": {"sessionId": session_id}})
        _, failed_closed = read_response(process, 9)
        if "result" not in failed_closed:
            raise RuntimeError(f"ACP session/close after provider failure failed: {failed_closed}")
        assert process.stdin is not None
        process.stdin.close()
        process.wait(timeout=10)

        process = launch()
        send(process, {"jsonrpc": "2.0", "id": 10, "method": "initialize", "params": {"protocolVersion": 1, "clientCapabilities": {}}})
        _, recovered_initialized = read_response(process, 10)
        if "result" not in recovered_initialized:
            raise RuntimeError(f"ACP recovery initialize failed: {recovered_initialized}")
        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 11,
                "method": "session/resume",
                "params": {"sessionId": session_id, "cwd": str(args.workspace), "mcpServers": []},
            },
        )
        _, recovered = read_response(process, 11)
        if "result" not in recovered:
            raise RuntimeError(f"ACP recovery session/resume failed: {recovered}")
        send(
            process,
            {
                "jsonrpc": "2.0",
                "id": 12,
                "method": "session/prompt",
                "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": "Reply with the recovery marker only."}]},
            },
        )
        recovered_lines, recovered_prompt = read_response(process, 12)
        recovered_output = "\n".join(recovered_lines)
        if "result" not in recovered_prompt or "LINUX_CARRIER_OK" not in recovered_output:
            raise RuntimeError(f"ACP recovery prompt failed: {recovered_prompt}; output={recovered_output}")
        if len(MockDeepSeek.requests) != 4:
            raise RuntimeError(f"expected four mock provider requests after recovery, got {len(MockDeepSeek.requests)}")
        print(json.dumps({"initialize": "ok", "session_new": "ok", "prompt": "ok", "resume": "ok", "provider_error": "ok", "recovery": "ok", "provider_requests": 4}))
        return 0
    finally:
        try:
            if session_id:
                send(process, {"jsonrpc": "2.0", "id": 4, "method": "session/close", "params": {"sessionId": session_id}})
        except (BrokenPipeError, OSError):
            pass
        if process.stdin is not None:
            process.stdin.close()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        server.shutdown()


if __name__ == "__main__":
    raise SystemExit(main())
