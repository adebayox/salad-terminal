package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustAndIgnore(t *testing.T) {
	dir := t.TempDir()
	if IsTrusted(dir) {
		t.Fatal("expected untrusted")
	}
	if err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(dir) {
		t.Fatal("expected trusted")
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600); err != nil {
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
