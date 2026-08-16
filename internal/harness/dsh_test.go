package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACP"}, Cwd: t.TempDir(), SessionID: "saved-session",
		Env: []string{"SALAD_DSH_TEST_HELPER=acp"}, Input: strings.NewReader("n\n"), Output: &output,
	}, "Make a safe change")
	if err != nil {
		t.Fatalf("RunACP() error = %v", err)
	}
	if result.SessionID != "fake-acp-session" {
		t.Fatalf("RunACP() session = %q", result.SessionID)
	}
	if !strings.Contains(output.String(), "ACP fake response") || !strings.Contains(output.String(), "Allow once?") || !strings.Contains(output.String(), "fresh continuation") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunACPUsesAdvertisedSessionRestore(t *testing.T) {
	var output strings.Builder
	var callbackSession string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunACP(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACPResume"}, Cwd: t.TempDir(), SessionID: "saved-session",
		Env: []string{"SALAD_DSH_TEST_HELPER=acp-resume"}, Input: strings.NewReader(""), Output: &output,
		OnSessionID: func(id string) { callbackSession = id },
	}, "Continue the work")
	if err != nil {
		t.Fatalf("RunACP() error = %v", err)
	}
	if result.SessionID != "saved-session" || callbackSession != result.SessionID || !strings.Contains(output.String(), "ACP resumed response") {
		t.Fatalf("session = %q output = %q", result.SessionID, output.String())
	}
}

func TestRunACPInteractiveKeepsOneSessionAcrossPrompts(t *testing.T) {
	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunACPInteractive(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACPInteractive"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp-interactive"}, Input: strings.NewReader("first request\nsecond request\n"), Output: &output,
	}, "")
	if err != nil {
		t.Fatalf("RunACPInteractive() error = %v", err)
	}
	if result.SessionID != "interactive-acp-session" {
		t.Fatalf("RunACPInteractive() session = %q", result.SessionID)
	}
	if !strings.Contains(output.String(), "response 1") || !strings.Contains(output.String(), "response 2") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestStartSessionStreamsIntoClientAndAcceptsPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := StartSession(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACP"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp"},
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	defer session.Close()

	promptDone := make(chan error, 1)
	go func() { promptDone <- session.Prompt(ctx, "Make a safe change") }()

	var gotAssistant, gotPermission bool
	for !(gotAssistant && gotPermission) {
		select {
		case event := <-session.Events():
			switch event.Kind {
			case "permission":
				gotPermission = true
				session.RespondPermission(event.PermissionID, false)
			case "assistant":
				gotAssistant = strings.Contains(event.Text, "ACP fake response")
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for session events")
		}
	}
	if err := <-promptDone; err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
}

func TestStartSessionCloseCancelsPendingPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := StartSession(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACP"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp"},
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	promptDone := make(chan error, 1)
	go func() { promptDone <- session.Prompt(ctx, "Close while approval is open") }()
	select {
	case event := <-session.Events():
		if event.Kind == "ready" {
			select {
			case event = <-session.Events():
			case <-ctx.Done():
				t.Fatal("timed out waiting for permission")
			}
		}
		if event.Kind != "permission" {
			t.Fatalf("first non-ready event = %#v, want permission", event)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for permission")
	}

	started := time.Now()
	session.Close()
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("Close() took %s while permission was pending", elapsed)
	}
	select {
	case err := <-promptDone:
		if err == nil {
			t.Fatal("Prompt() succeeded after the session was closed")
		}
	case <-ctx.Done():
		t.Fatal("Prompt() did not return after Close()")
	}
}

func TestRunACPStartsFreshWhenSessionIDWasNotRequested(t *testing.T) {
	var output strings.Builder
	var callbackSession string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunACP(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACPResume"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp-resume"}, Output: &output,
		OnSessionID: func(id string) { callbackSession = id },
	}, "Start new work")
	if err != nil {
		t.Fatalf("RunACP() error = %v", err)
	}
	if result.SessionID != "fresh-acp-session" || callbackSession != result.SessionID || !strings.Contains(output.String(), "fresh response") {
		t.Fatalf("session = %q output = %q", result.SessionID, output.String())
	}
	if strings.Contains(output.String(), "carrier does not advertise session restore") {
		t.Fatalf("fresh run incorrectly reported a restore fallback: %q", output.String())
	}
}

func TestRunACPPromptTimeoutCancelsStalledTurn(t *testing.T) {
	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := RunACP(ctx, Options{
		Command: os.Args[0], Args: []string{"-test.run=TestHarnessFakeACPTimeout"}, Cwd: t.TempDir(),
		Env: []string{"SALAD_DSH_TEST_HELPER=acp-timeout"}, Output: &output, PromptTimeout: 50 * time.Millisecond,
	}, "This turn must time out")
	if err == nil || !strings.Contains(err.Error(), "ACP prompt timed out") {
		t.Fatalf("RunACP() error = %v, output = %q", err, output.String())
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

func TestScrubbedEnvironmentDoesNotInheritNetworkAllow(t *testing.T) {
	t.Setenv("DSH_NETWORK_MODE", "allow")
	env := scrubbedEnvironment(withHarnessSafetyDefaults(nil, "/tmp/workspace"))
	if !containsEnvironment(env, "DSH_NETWORK_MODE=deny") || containsEnvironment(env, "DSH_NETWORK_MODE=allow") {
		t.Fatalf("network capability inherited from parent: %v", env)
	}
	env = scrubbedEnvironment(withHarnessSafetyDefaults([]string{"DSH_NETWORK_MODE=allow"}, "/tmp/workspace"))
	if !containsEnvironment(env, "DSH_NETWORK_MODE=allow") || containsEnvironment(env, "DSH_NETWORK_MODE=deny") {
		t.Fatalf("explicit network capability was not preserved: %v", env)
	}
	env = scrubbedEnvironment(withHarnessSafetyDefaults([]string{"DSH_NETWORK_MODE=loopback"}, "/tmp/workspace"))
	if !containsEnvironment(env, "DSH_NETWORK_MODE=loopback") || containsEnvironment(env, "DSH_NETWORK_MODE=allow") {
		t.Fatalf("explicit loopback capability was not preserved safely: %v", env)
	}
}

func TestHarnessSafetyDefaultsDenyNetworkUnlessExplicitlyOverridden(t *testing.T) {
	defaulted := withHarnessSafetyDefaults(nil, "/tmp/workspace")
	if !containsEnvironment(defaulted, "DSH_NETWORK_MODE=deny") || !containsEnvironment(defaulted, "DSH_CWD=/tmp/workspace") {
		t.Fatalf("defaults = %v", defaulted)
	}
	overridden := withHarnessSafetyDefaults([]string{"DSH_NETWORK_MODE=allow"}, "/tmp/workspace")
	if !containsEnvironment(overridden, "DSH_NETWORK_MODE=allow") || containsEnvironment(overridden, "DSH_NETWORK_MODE=deny") {
		t.Fatalf("override = %v", overridden)
	}
	loopback := withHarnessSafetyDefaults([]string{"DSH_NETWORK_MODE=loopback"}, "/tmp/workspace")
	if !containsEnvironment(loopback, "DSH_NETWORK_MODE=loopback") || containsEnvironment(loopback, "DSH_NETWORK_MODE=deny") {
		t.Fatalf("loopback override = %v", loopback)
	}
}

func containsEnvironment(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func TestHarnessFakeACPResume(t *testing.T) {
	if os.Getenv("SALAD_DSH_TEST_HELPER") != "acp-resume" {
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
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": map[string]any{"loadSession": true},
			}})
		case "session/load":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{}})
		case "session/new":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]string{"sessionId": "fresh-acp-session"}})
		case "session/prompt":
			params, _ := frame["params"].(map[string]any)
			sessionID, _ := params["sessionId"].(string)
			response := "ACP resumed response"
			if sessionID == "fresh-acp-session" {
				response = "fresh response"
			}
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
				"sessionId": sessionID, "update": map[string]any{
					"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": response},
				},
			}})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "result": map[string]string{"stopReason": "end_turn"}})
			return
		}
	}
}

func TestHarnessFakeACPTimeout(t *testing.T) {
	if os.Getenv("SALAD_DSH_TEST_HELPER") != "acp-timeout" {
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
		id := frame["id"]
		switch method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"protocolVersion": 1}})
		case "session/new":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]string{"sessionId": "timeout-acp-session"}})
		case "session/prompt":
			for {
				time.Sleep(time.Hour)
			}
		}
	}
}

func TestHarnessFakeACPInteractive(t *testing.T) {
	if os.Getenv("SALAD_DSH_TEST_HELPER") != "acp-interactive" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	promptCount := 0
	for scanner.Scan() {
		var frame map[string]any
		if json.Unmarshal(scanner.Bytes(), &frame) != nil {
			continue
		}
		method, _ := frame["method"].(string)
		id := frame["id"]
		switch method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
				"protocolVersion": 1, "agentCapabilities": map[string]any{"sessionCapabilities": map[string]any{"close": map[string]any{}}},
			}})
		case "session/new":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]string{"sessionId": "interactive-acp-session"}})
		case "session/prompt":
			promptCount++
			params, _ := frame["params"].(map[string]any)
			sessionID, _ := params["sessionId"].(string)
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
				"sessionId": sessionID, "update": map[string]any{
					"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": fmt.Sprintf("response %d", promptCount)},
				},
			}})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]string{"stopReason": "end_turn"}})
		case "session/close":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}})
			return
		}
	}
}
