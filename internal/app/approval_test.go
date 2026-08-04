package app

import (
	"testing"

	"github.com/salad-ai/salad-terminal/internal/tools"
)

func TestApprovePendingDequeuesTheRequest(t *testing.T) {
	m := model{
		pendingTool:    &tools.Request{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"},
		pendingKind:    "edit",
		screen:         screenApprove,
		toolQueue:      []tools.Request{{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"}},
		pendingPreview: "diff",
		pendingReason:  "review",
	}
	next, cmd := m.approvePending(false)
	if len(next.toolQueue) != 0 {
		t.Fatalf("approve must dequeue the pending request, got %d queued", len(next.toolQueue))
	}
	if next.pendingTool != nil {
		t.Fatal("approve must clear pendingTool")
	}
	if next.screen != screenRoom {
		t.Fatalf("approve must return to room, got screen %d", next.screen)
	}
	if cmd == nil {
		t.Fatal("approve must issue an execute command")
	}
}

func TestApprovePendingKeepsLaterQueuedRequests(t *testing.T) {
	m := model{
		pendingTool: &tools.Request{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"},
		pendingKind: "edit",
		screen:      screenApprove,
		toolQueue: []tools.Request{
			{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"},
			{RequestID: "r2", ToolCallID: "t2", ToolName: "run_command"},
		},
	}
	next, _ := m.approvePending(false)
	if len(next.toolQueue) != 1 || next.toolQueue[0].RequestID != "r2" {
		t.Fatalf("approve must only remove the pending (front) request, got %#v", next.toolQueue)
	}
}

func TestRejectPendingDequeuesTheRequest(t *testing.T) {
	m := model{
		pendingTool: &tools.Request{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"},
		pendingKind: "edit",
		screen:      screenApprove,
		toolQueue:   []tools.Request{{RequestID: "r1", ToolCallID: "t1", ToolName: "apply_edit"}},
	}
	next, cmd := m.rejectPending()
	if len(next.toolQueue) != 0 {
		t.Fatalf("reject must dequeue the pending request, got %d queued", len(next.toolQueue))
	}
	if next.pendingTool != nil {
		t.Fatal("reject must clear pendingTool")
	}
	if cmd == nil {
		t.Fatal("reject must issue a post-error command")
	}
}

func TestApproveWithNoPendingIsNoOp(t *testing.T) {
	m := model{screen: screenApprove}
	next, cmd := m.approvePending(false)
	if cmd != nil {
		t.Fatal("no pending tool must not issue a command")
	}
	if next.screen != screenApprove {
		t.Fatal("no pending tool must not change screen")
	}
}
