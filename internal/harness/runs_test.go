package harness

import (
	"path/filepath"
	"strings"
	"testing"
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
