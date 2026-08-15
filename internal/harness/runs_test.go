package harness

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionRootIsStableAndDoesNotEmbedWorkspacePath(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	workspace := filepath.Join(t.TempDir(), "repo", "with spaces")
	first, err := SessionRoot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SessionRoot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("session roots differ: %q vs %q", first, second)
	}
	if strings.Contains(first, workspace) || !strings.HasSuffix(first, filepath.Join("sessions", filepath.Base(first))) {
		t.Fatalf("session root leaks or has unexpected shape: %q", first)
	}
}

func TestListRunsReturnsNewestFirstAndPreservesState(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	workspace := filepath.Join(t.TempDir(), "repo")
	older := RunRecord{
		ID: "salad-engineer-old", Workspace: workspace, Protocol: "acp", Prompt: "old",
		StartedAt: time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 15, 10, 1, 0, 0, time.UTC), Status: "completed",
	}
	newer := RunRecord{
		ID: "salad-engineer-new", Workspace: workspace, Protocol: "acp", Prompt: "new",
		StartedAt: time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 15, 11, 1, 0, 0, time.UTC), Status: "running", SessionID: "session-123",
	}
	if err := SaveRun(older); err != nil {
		t.Fatal(err)
	}
	if err := SaveRun(newer); err != nil {
		t.Fatal(err)
	}
	runs, err := ListRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != newer.ID || runs[0].Status != "running" || runs[0].SessionID != newer.SessionID {
		t.Fatalf("ListRuns() = %#v, want newest state first", runs)
	}
}
