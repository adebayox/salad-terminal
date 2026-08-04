package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustAndIgnore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())

	if IsTrusted(dir) {
		t.Fatal("expected untrusted")
	}
	if err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(dir) {
		t.Fatal("expected trusted after Trust()")
	}

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}

	content, err := ReadFile(dir, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello" {
		t.Fatalf("got %q", content)
	}
	if _, err := ReadFile(dir, ".env"); err == nil {
		t.Fatal("expected .env blocked")
	}
}

func TestSymlinkEscapeBlocked(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "escape"); err == nil {
		t.Fatal("expected symlink escape to be blocked")
	}
}

func TestListAndSearch(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Auth() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := ListDirectory(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "main.go") {
		t.Fatalf("list missing main.go: %s", list)
	}
	hits, err := SearchCodebase(root, "Auth", "*.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hits, "main.go") {
		t.Fatalf("search missed Auth: %s", hits)
	}
}

func TestLegacyTrustFileStillHonored(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".salad-trust"), []byte("trusted=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(root) {
		t.Fatal("expected legacy .salad-trust to grant trust")
	}
}

func TestTrustViaSymlinkedPathMatchesPhysicalPath(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())

	// Real workspace dir and a symlink alias to it (like /tmp -> /private/tmp on macOS).
	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "workspace")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// Trust through the logical symlink path; physical access must still match.
	if err := Trust(link); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(link) {
		t.Fatal("expected trusted via the same logical path")
	}
	resolved, err := ResolveRoot(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(resolved) {
		t.Fatalf("expected trust recorded via symlink to match physical root %q", resolved)
	}
}
