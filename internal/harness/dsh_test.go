package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunWithFakeRuntime(t *testing.T) {
	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := Run(ctx, Options{Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeRuntime"}, Cwd: t.TempDir(), Env: []string{"SALAD_DSH_TEST_HELPER=1"}, Output: &output}, "Explain the test repository")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.SessionID == "" {
		t.Fatal("Run() returned an empty session ID")
	}
	if !strings.Contains(output.String(), "fake harness response") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunACPWithFakeRuntimeUsesNumericIDsAndRejectsApproval(t *testing.T) {
	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunACP(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACP"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp"}, Input: strings.NewReader("n\n"), Output: &output,
	}, "Make a safe change")
	if err != nil {
		t.Fatalf("RunACP() error = %v", err)
	}
	if result.SessionID != "fake-acp-session" {
		t.Fatalf("RunACP() session = %q", result.SessionID)
	}
	if !strings.Contains(output.String(), "ACP fake response") || !strings.Contains(output.String(), "Allow once?") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunCancellationReapsChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, Options{Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeRuntime"}, Cwd: t.TempDir(), Env: []string{"SALAD_DSH_TEST_HELPER=1", "SALAD_DSH_TEST_MODE=block"}}, "wait")
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() succeeded after cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not reap the cancelled child")
	}
}

func TestDecodeFrameRejectsMalformedAndOversized(t *testing.T) {
	if _, err := decodeFrame([]byte("not json")); err == nil {
		t.Fatal("accepted malformed JSON")
	}
	if _, err := decodeFrame([]byte(strings.Repeat("x", maxFrameBytes+1))); err == nil {
		t.Fatal("accepted oversized frame")
	}
}

func TestScrubbedEnvironment(t *testing.T) {
	env := scrubbedEnvironment([]string{"SALAD_ACCESS_TOKEN=do-not-forward", "SALAD_REFRESH_TOKEN=do-not-forward", "DEEPSEEK_API_KEY=explicit-runtime-key", "DSH_CORDIS_CONFIG=/tmp/config.yml"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "do-not-forward") {
		t.Fatalf("Salad credential leaked: %q", joined)
	}
	if !strings.Contains(joined, "DEEPSEEK_API_KEY=explicit-runtime-key") || !strings.Contains(joined, "DSH_CORDIS_CONFIG=/tmp/config.yml") {
		t.Fatalf("runtime configuration missing: %q", joined)
	}
}

func TestTerminalSafeRemovesControlSequences(t *testing.T) {
	got := terminalSafe("ok\x1b]0;evil\x07\x1b[31mred\x1b[0m\x01")
	if got != "okred" {
		t.Fatalf("terminalSafe() = %q", got)
	}
}

func TestHarnessFakeRuntime(t *testing.T) {
	if os.Getenv("SALAD_DSH_TEST_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var frame map[string]any
		if json.Unmarshal(scanner.Bytes(), &frame) != nil {
			continue
		}
		method, _ := frame["method"].(string)
		id, _ := frame["id"].(string)
		switch method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"serverInfo": map[string]string{"name": "fake", "version": "test"}}})
		case "session/prompt":
			if os.Getenv("SALAD_DSH_TEST_MODE") == "block" {
				select {}
			}
			params, _ := frame["params"].(map[string]any)
			sessionID, _ := params["sessionId"].(string)
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session.event", "params": map[string]any{"sessionId": sessionID, "event": map[string]any{"type": "assistant/message", "data": map[string]any{"message": map[string]any{"content": []map[string]string{{"type": "text", "text": "fake harness response"}}}}}}})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session.status", "params": map[string]string{"sessionId": sessionID, "status": "idle"}})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]string{"messageId": "fake-message"}})
		case "shutdown":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}})
			return
		}
	}
}

func TestHarnessFakeACP(t *testing.T) {
	if os.Getenv("SALAD_DSH_TEST_HELPER") != "acp" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var frame map[string]any
		if json.Unmarshal(scanner.Bytes(), &frame) != nil {
			continue
		}
		method, _ := frame["method"].(string)
		switch method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"protocolVersion": 1}})
		case "session/new":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]string{"sessionId": "fake-acp-session"}})
		case "session/prompt":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0", "id": 99, "method": "session/request_permission",
				"params": map[string]string{"sessionId": "fake-acp-session"},
			})
			if !scanner.Scan() {
				return
			}
			var decision map[string]any
			if json.Unmarshal(scanner.Bytes(), &decision) != nil {
				return
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0", "method": "session/update",
				"params": map[string]any{"sessionId": "fake-acp-session", "update": map[string]any{
					"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "ACP fake response"},
				}},
			})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "result": map[string]string{"stopReason": "end_turn"}})
			return
		}
	}
}
