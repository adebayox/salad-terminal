#!/usr/bin/env python3
"""Run a provider-free ACP boot/prompt smoke against a packaged DSH carrier."""

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
        self.requests.append(json.loads(body))
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
    process = subprocess.Popen(
        [str(args.carrier), "--config", str(args.config)],
        cwd=args.workspace,
        env=env,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )
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
        print(json.dumps({"initialize": "ok", "session_new": "ok", "prompt": "ok", "provider_requests": 1}))
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
