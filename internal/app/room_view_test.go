package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/salad-ai/salad-terminal/internal/harness"
)

func TestRoomViewKeepsOneTerminalSurfaceForChatAndWorkspace(t *testing.T) {
	m := newModel(Options{ForceContinue: true})
	m.screen = screenRoom
	m.width = 120
	m.chatTitle = "saladBE"

	view := m.viewRoom()
	if !strings.Contains(view, "Salad chat") {
		t.Fatalf("room header does not identify the normal chat path: %q", view)
	}
	if strings.Contains(view, "salad engineer") {
		t.Fatalf("normal chat view advertises a separate engineer command: %q", view)
	}

	m.workspaceMode = true
	m.workspaceOK = true
	workspaceView := m.viewRoom()
	if !strings.Contains(workspaceView, "Salad Terminal") || !strings.Contains(workspaceView, "DeepSeek Harness") {
		t.Fatalf("workspace view does not identify the integrated agent: %q", workspaceView)
	}
	if !strings.Contains(workspaceView, "/chat normal chat") {
		t.Fatalf("workspace view does not provide the normal chat escape hatch: %q", workspaceView)
	}
}

func TestWorkspaceOffersNormalChatWithoutSecondTerminal(t *testing.T) {
	m := newModel(Options{WorkspaceMode: true})
	m.screen = screenRoom
	m.workspaceMode = true
	updated, _ := m.runSlash("/chat")
	next := updated.(model)
	if next.workspaceMode {
		t.Fatal("/chat left the workspace agent mode enabled")
	}
	if next.screen != screenNewAI {
		t.Fatalf("/chat screen = %v, want normal chat picker", next.screen)
	}
}

func TestWorkspacePromptUsesExistingTerminalTranscript(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runtimePath := filepath.Join(t.TempDir(), "fake-acp.sh")
	runtime := `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *initialize*) printf '%s\n' '{"jsonrpc":"2.0","id":"1","result":{"protocolVersion":1}}' ;;
    *session/new*) printf '%s\n' '{"jsonrpc":"2.0","id":"2","result":{"sessionId":"fake-session"}}' ;;
    *session/prompt*)
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"fake-session","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"response 1"}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","id":"3","result":{"stopReason":"end_turn"}}'
      ;;
    *session/close*) printf '%s\n' '{"jsonrpc":"2.0","id":"4","result":{}}' ; exit 0 ;;
  esac
done
`
	if err := os.WriteFile(runtimePath, []byte(runtime), 0o700); err != nil {
		t.Fatalf("write fake ACP runtime: %v", err)
	}
	session, err := harness.StartSession(ctx, harness.Options{
		Command: runtimePath, Cwd: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	defer session.Close()

	m := newModel(Options{WorkspaceMode: true})
	m.screen = screenRoom
	m.workspaceMode = true
	m.workspaceOK = true
	m.workspaceSession = session
	m.composer.SetValue("inspect this project")
	updatedModel, command := m.sendComposer()
	message := command()
	updatedModel, _ = updatedModel.(model).Update(message)
	updated := updatedModel.(model)

	for {
		select {
		case event := <-session.Events():
			updatedModel, _ = updated.Update(workspaceEventMsg{generation: updated.workspaceGeneration, event: event})
			updated = updatedModel.(model)
			if event.Kind == "turn_end" {
				if updated.sending {
					t.Fatal("workspace turn still marked sending after ACP turn_end")
				}
				if !strings.Contains(updated.messages[len(updated.messages)-1].Body, "response 1") {
					t.Fatalf("transcript = %#v", updated.messages)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for ACP transcript events")
		}
	}
}
