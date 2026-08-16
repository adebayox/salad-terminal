package app

import (
	"strings"
	"testing"
)

func TestRoomViewIdentifiesNormalChatAndEngineerCommand(t *testing.T) {
	m := newModel(Options{})
	m.screen = screenRoom
	m.width = 120
	m.chatTitle = "saladBE"

	view := m.viewRoom()
	if !strings.Contains(view, "Salad chat") {
		t.Fatalf("room header does not identify the normal chat path: %q", view)
	}
	if !strings.Contains(view, "codebase work: salad engineer") {
		t.Fatalf("room footer does not point developers to engineer mode: %q", view)
	}
}
