package app

import (
	"testing"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
)

func TestMergeMessagesKeepsOlderWhenPollReturnsWindow(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	existing := []api.ChatMessage{
		{ID: "1", Body: "first", CreatedAt: t0},
		{ID: "2", Body: "second", CreatedAt: t0.Add(time.Minute)},
		{ID: "3", Body: "third", CreatedAt: t0.Add(2 * time.Minute)},
	}
	// Server page only has the latest two (simulates bootstrap limit).
	incoming := []api.ChatMessage{
		{ID: "2", Body: "second", CreatedAt: t0.Add(time.Minute)},
		{ID: "3", Body: "third updated", CreatedAt: t0.Add(2 * time.Minute)},
		{ID: "4", Body: "fourth", CreatedAt: t0.Add(3 * time.Minute)},
	}

	got := mergeMessages(existing, incoming)
	if len(got) != 4 {
		t.Fatalf("expected 4 messages after merge, got %d", len(got))
	}
	if got[0].ID != "1" || got[0].Body != "first" {
		t.Fatalf("oldest message was dropped: %+v", got[0])
	}
	if got[2].Body != "third updated" {
		t.Fatalf("expected server body update, got %q", got[2].Body)
	}
	if got[3].ID != "4" {
		t.Fatalf("expected newest appended, got %+v", got[3])
	}
}

func TestPreferMessageKeepsLongerBody(t *testing.T) {
	a := api.ChatMessage{ID: "1", Body: "hello world", Status: "streaming"}
	b := api.ChatMessage{ID: "1", Body: "hi", Status: "done"}
	got := preferMessage(a, b)
	if got.Body != "hello world" {
		t.Fatalf("expected longer body kept, got %q", got.Body)
	}
}
