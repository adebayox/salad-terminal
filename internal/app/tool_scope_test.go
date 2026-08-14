package app

import (
	"strings"
	"testing"

	"github.com/salad-ai/salad-terminal/internal/tools"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

func TestValidateToolRequestScopeRequiresActiveChatAndWorkspace(t *testing.T) {
	root := t.TempDir()
	id, err := workspace.OpaqueID(root)
	if err != nil {
		t.Fatal(err)
	}
	req := tools.Request{RequestID: "request-1", ToolCallID: "tool-1", WorkspaceID: id}

	cases := []struct {
		name      string
		screen    screen
		eventChat string
		workspace string
		trusted   bool
		want      string
	}{
		{"wrong screen", screenChats, "chat-1", id, true, "outside the active chat"},
		{"wrong chat", screenRoom, "chat-2", id, true, "outside the active chat"},
		{"untrusted", screenRoom, "chat-1", id, false, "workspace tools are not enabled"},
		{"wrong workspace", screenRoom, "chat-1", "other", true, "another workspace"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testReq := req
			testReq.WorkspaceID = tc.workspace
			err := validateToolRequestScope(tc.screen, "chat-1", root, tc.eventChat, tc.trusted, testReq)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := validateToolRequestScope(screenRoom, "chat-1", root, "chat-1", true, req); err != nil {
		t.Fatalf("valid tool request rejected: %v", err)
	}
}
