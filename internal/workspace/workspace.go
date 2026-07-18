package workspace

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const trustFileName = ".salad-trust"

// ResolveRoot returns an absolute workspace root.
func ResolveRoot(explicit string) (string, error) {
	root := explicit
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	return filepath.Abs(root)
}

func trustPath(root string) string {
	return filepath.Join(root, trustFileName)
}

func IsTrusted(root string) bool {
	_, err := os.Stat(trustPath(root))
	return err == nil
}

func Trust(root string) error {
	root, err := ResolveRoot(root)
	if err != nil {
		return err
	}
	content := "trusted=true\n# Local Salad Terminal workspace trust marker.\n# Delete this file to revoke trust.\n"
	return os.WriteFile(trustPath(root), []byte(content), 0o600)
}

func EnsureTrusted(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	if IsTrusted(root) {
		return root, nil
	}
	fmt.Printf("Trust this workspace for local Salad tools?\n  %s\n[y/N] ", root)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return "", fmt.Errorf("workspace not trusted")
	}
	if err := Trust(root); err != nil {
		return "", err
	}
	fmt.Println("Trusted.")
	return root, nil
}

// RequireTrusted returns the root only if already trusted (never prompts).
func RequireTrusted(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	if !IsTrusted(root) {
		return "", fmt.Errorf("workspace not trusted (run: salad workspace trust, or /trust in TUI)")
	}
	return root, nil
}

func loadSaladIgnore(root string) []string {
	defaults := []string{
		".env", ".env.*", "*.pem", "*.key", "credentials.json",
		"**/node_modules/**", "**/.git/objects/**",
	}
	path := filepath.Join(root, ".saladignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults
	}
	lines := defaults
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isIgnored(root, rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		if matched, _ := filepath.Match(pattern, rel); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if strings.HasPrefix(pattern, "**/") {
			suffix := strings.TrimPrefix(pattern, "**/")
			if matched, _ := filepath.Match(suffix, base); matched {
				return true
			}
			if strings.Contains(rel, strings.TrimSuffix(suffix, "/**")) {
				return true
			}
		}
	}
	return false
}

// ReadFile reads a workspace-relative file if trusted and not ignored.
func ReadFile(root, relPath string) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	relPath = filepath.Clean(relPath)
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path escapes workspace")
	}
	if isIgnored(root, relPath, loadSaladIgnore(root)) {
		return "", fmt.Errorf("path blocked by .saladignore / defaults: %s", relPath)
	}
	abs := filepath.Join(root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	const maxBytes = 256 * 1024
	if len(data) > maxBytes {
		return string(data[:maxBytes]) + "\n… truncated …\n", nil
	}
	return string(data), nil
}

func GitStatus(root string) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	return runGit(root, "status", "--short", "--branch")
}

func GitDiff(root string) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	return runGit(root, "diff", "--stat")
}

func PermissionsSummary(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "workspace: %s\n", root)
	fmt.Fprintf(&b, "trusted: %v\n", IsTrusted(root))
	fmt.Fprintf(&b, "allowed tools: read, git-status, git-diff, permissions\n")
	fmt.Fprintf(&b, "denied: shell exec, network tools, MCP\n")
	fmt.Fprintf(&b, "terminal turns may attach code_context when trusted\n")
	fmt.Fprintf(&b, "ignore patterns:\n")
	for _, p := range loadSaladIgnore(root) {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	return b.String(), nil
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
