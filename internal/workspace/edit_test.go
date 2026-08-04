package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func trustRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUnifiedDiff(t *testing.T) {
	cases := []struct {
		name     string
		oldText  string
		newText  string
		wantSub  string // substring that must appear
		wantNot  string // substring that must NOT appear
		noChange bool
	}{
		{name: "no change", oldText: "a\nb\n", newText: "a\nb\n", noChange: true},
		{name: "new file", oldText: "", newText: "line1\n", wantSub: "+line1", wantNot: "-line2"},
		{name: "delete", oldText: "line1\nline2\n", newText: "", wantSub: "-line1"},
		{name: "single edit", oldText: "x\nkeep\nkeep\nold\nkeep\nkeep\ny\n", newText: "x\nkeep\nkeep\nnew\nkeep\nkeep\ny\n", wantSub: "-old"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := UnifiedDiff("f.txt", tc.oldText, tc.newText)
			if err != nil {
				t.Fatal(err)
			}
			if tc.noChange {
				if !strings.Contains(diff, "no changes") {
					t.Fatalf("expected no-change diff, got:\n%s", diff)
				}
				return
			}
			if !strings.Contains(diff, tc.wantSub) {
				t.Fatalf("diff missing %q:\n%s", tc.wantSub, diff)
			}
			if tc.wantNot != "" && strings.Contains(diff, tc.wantNot) {
				t.Fatalf("diff should not contain %q:\n%s", tc.wantNot, diff)
			}
			if !strings.Contains(diff, "--- a/f.txt") || !strings.Contains(diff, "+++ b/f.txt") {
				t.Fatalf("diff missing headers:\n%s", diff)
			}
		})
	}
}

func TestApplyEditCreatesAndModifies(t *testing.T) {
	root := trustRoot(t)

	created, err := ApplyEdit(root, "src/app.go", "package app\n", "scaffold")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created, "src/app.go") {
		t.Fatalf("apply result missing path: %s", created)
	}
	data, err := os.ReadFile(filepath.Join(root, "src/app.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package app\n" {
		t.Fatalf("content = %q", string(data))
	}

	modified, err := ApplyEdit(root, "src/app.go", "package app\n\nfunc Hello() {}\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modified, "+func Hello") {
		t.Fatalf("diff missing addition: %s", modified)
	}
}

func TestApplyEditPreservesMode(t *testing.T) {
	root := trustRoot(t)
	path := filepath.Join(root, "exec.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyEdit(root, "exec.sh", "#!/bin/sh\necho hi\n", ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %o, want 755", info.Mode().Perm())
	}
}

func TestApplyEditBlocksIgnoredAndEscapes(t *testing.T) {
	root := trustRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyEdit(root, ".env", "SECRET=2\n", ""); err == nil {
		t.Fatal("expected .env edit to be blocked")
	}
	if _, err := ApplyEdit(root, "../outside.txt", "x\n", ""); err == nil {
		t.Fatal("expected escape to be blocked")
	}
}

func TestPreviewEditDoesNotWrite(t *testing.T) {
	root := trustRoot(t)
	path := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := PreviewEdit(root, "keep.txt", "after\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "-before") || !strings.Contains(diff, "+after") {
		t.Fatalf("preview diff wrong:\n%s", diff)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "before\n" {
		t.Fatalf("preview wrote to disk: %q", string(data))
	}
}

func TestRunCommandBounded(t *testing.T) {
	root := trustRoot(t)

	out, err := RunCommand(root, []string{"printf", "hello"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("output = %q", out)
	}

	if _, err := RunCommand(root, []string{"false"}, ""); err == nil {
		t.Fatal("expected exit-code error for false")
	}

	if _, err := RunCommand(root, []string{"pwd"}, "../escape"); err == nil {
		t.Fatal("expected cwd escape to be rejected")
	}
}

func TestSanitizedEnvStripsSecrets(t *testing.T) {
	t.Setenv("SALAD_TEST_TOKEN", "abc")
	t.Setenv("SALAD_TEST_PLAIN", "ok")
	env := sanitizedEnv()
	seen := map[string]string{}
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			seen[kv[:i]] = kv[i+1:]
		}
	}
	if v, ok := seen["SALAD_TEST_TOKEN"]; ok && v != "" {
		t.Fatalf("secret env var leaked: %s", v)
	}
	if seen["SALAD_TEST_PLAIN"] != "ok" {
		t.Fatalf("non-secret env var dropped: %q", seen["SALAD_TEST_PLAIN"])
	}
}

func TestProjectMemory(t *testing.T) {
	root := trustRoot(t)
	if _, err := ProjectMemory(root); err == nil {
		t.Fatal("expected error when no memory file")
	}
	if err := os.WriteFile(filepath.Join(root, "SALAD.md"), []byte("Use Go idioms.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("Never use panic.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mem, err := ProjectMemory(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mem, "SALAD.md") || !strings.Contains(mem, "CLAUDE.md") {
		t.Fatalf("memory missing files:\n%s", mem)
	}
	if !strings.Contains(mem, "Go idioms") || !strings.Contains(mem, "Never use panic") {
		t.Fatalf("memory missing content:\n%s", mem)
	}
}
