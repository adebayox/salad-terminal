package workspace

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/salad-ai/salad-terminal/internal/config"
)

const trustFileName = ".salad-trust"

// ResolveRoot returns an absolute, symlink-canonicalized workspace root.
// Canonicalization keeps trust keys consistent: os.Getwd() already returns the
// physical path on macOS/Linux (e.g. /private/tmp/... when launched via
// /tmp/...), so a trust entry recorded through the logical path must resolve to
// the same key or IsTrusted silently misses.
func ResolveRoot(explicit string) (string, error) {
	root := explicit
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return abs, nil
}

// OpaqueID returns a stable non-path workspace identifier for server bindings.
func OpaqueID(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:16]), nil
}

func trustPath(root string) string {
	return filepath.Join(root, trustFileName)
}

func trustedWorkspacesPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "trusted_workspaces.json"), nil
}

func loadTrustedWorkspaces() (map[string]bool, error) {
	path, err := trustedWorkspacesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]bool{}
	}
	return out, nil
}

func saveTrustedWorkspaces(set map[string]bool) error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := trustedWorkspacesPath()
	if err != nil {
		return err
	}
	if set == nil {
		set = map[string]bool{}
	}
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func IsTrusted(root string) bool {
	root, err := ResolveRoot(root)
	if err != nil {
		return false
	}
	if set, err := loadTrustedWorkspaces(); err == nil && set[root] {
		return true
	}
	// Backward compatible: in-repo marker from earlier Terminal builds.
	_, err = os.Stat(trustPath(root))
	return err == nil
}

func Trust(root string) error {
	root, err := ResolveRoot(root)
	if err != nil {
		return err
	}
	set, err := loadTrustedWorkspaces()
	if err != nil {
		return err
	}
	set[root] = true
	return saveTrustedWorkspaces(set)
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

// resolveInsideWorkspace joins root+rel, resolves symlinks, and re-checks the boundary.
func resolveInsideWorkspace(root, relPath string) (abs string, rel string, err error) {
	root, err = RequireTrusted(root)
	if err != nil {
		return "", "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	relPath = filepath.Clean(relPath)
	if relPath == "." {
		relPath = ""
	}
	if strings.HasPrefix(relPath, "..") {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	candidate := resolvedRoot
	if relPath != "" {
		candidate = filepath.Join(resolvedRoot, relPath)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Allow missing leaf for list/read errors to surface later, but only if
		// the parent stays inside the workspace.
		parent := filepath.Dir(candidate)
		parentResolved, parentErr := filepath.EvalSymlinks(parent)
		if parentErr != nil {
			return "", "", fmt.Errorf("path escapes workspace")
		}
		if !isInsideRoot(resolvedRoot, parentResolved) {
			return "", "", fmt.Errorf("path escapes workspace")
		}
		resolved = candidate
	}
	if !isInsideRoot(resolvedRoot, resolved) {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	rel, err = filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	return resolved, filepath.ToSlash(rel), nil
}

func isInsideRoot(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	prefix := root + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}

// ReadFile reads a workspace-relative file if trusted and not ignored.
func ReadFile(root, relPath string) (string, error) {
	abs, rel, err := resolveInsideWorkspace(root, relPath)
	if err != nil {
		return "", err
	}
	if isIgnored(root, rel, loadSaladIgnore(root)) {
		return "", fmt.Errorf("path blocked by .saladignore / defaults: %s", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	const maxBytes = 256 * 1024
	if len(data) > maxBytes {
		data = data[:maxBytes]
		return string(data) + "\n… truncated …\n", nil
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text: %s", rel)
	}
	return string(data), nil
}

// ListDirectory lists entries under a relative workspace path.
func ListDirectory(root, relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		relPath = "."
	}
	abs, rel, err := resolveInsideWorkspace(root, relPath)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	patterns := loadSaladIgnore(root)
	var b strings.Builder
	fmt.Fprintf(&b, "%s/\n", rel)
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if rel != "." && rel != "" {
			childRel = rel + "/" + name
		}
		if isIgnored(root, childRel, patterns) {
			continue
		}
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		fmt.Fprintf(&b, "  %s%s\n", name, suffix)
		count++
		if count >= 500 {
			b.WriteString("  … truncated …\n")
			break
		}
	}
	return b.String(), nil
}

// SearchCodebase walks the workspace for query matches (bounded).
func SearchCodebase(root, query, filePattern string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query required")
	}
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	patterns := loadSaladIgnore(root)
	filePattern = strings.TrimSpace(filePattern)
	var b strings.Builder
	matches := 0
	const maxMatches = 40
	err = filepath.WalkDir(resolvedRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !isInsideRoot(resolvedRoot, path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(resolvedRoot, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" {
				return fs.SkipDir
			}
			if isIgnored(root, relSlash, patterns) {
				return fs.SkipDir
			}
			return nil
		}
		if isIgnored(root, relSlash, patterns) {
			return nil
		}
		if filePattern != "" {
			ok, _ := filepath.Match(filePattern, filepath.Base(relSlash))
			ok2, _ := filepath.Match(filePattern, relSlash)
			if !ok && !ok2 {
				return nil
			}
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() > 512*1024 {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || !utf8.Valid(data) {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		lowerQuery := strings.ToLower(query)
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				fmt.Fprintf(&b, "%s:%d:%s\n", relSlash, i+1, trimRunes(line, 200))
				matches++
				if matches >= maxMatches {
					b.WriteString("… truncated …\n")
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if matches == 0 {
		return "no matches", nil
	}
	return b.String(), nil
}

func trimRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}

func GitStatus(root string) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	return runGit(root, "status", "--short", "--branch")
}

func GitDiff(root string, path string, statOnly bool) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	args := []string{"diff"}
	if statOnly {
		args = append(args, "--stat")
	}
	path = strings.TrimSpace(path)
	if path != "" {
		if _, _, err := resolveInsideWorkspace(root, path); err != nil {
			return "", err
		}
		args = append(args, "--", path)
	}
	out, err := runGit(root, args...)
	if err != nil {
		return "", err
	}
	return trimOutput(out, 80_000), nil
}

func GitLog(root string, limit int) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}
	return runGit(root, "log", fmt.Sprintf("-%d", limit), "--oneline", "--decorate")
}

func PermissionsSummary(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	id, _ := OpaqueID(root)
	var b strings.Builder
	fmt.Fprintf(&b, "workspace_id: %s\n", id)
	fmt.Fprintf(&b, "trusted: %v\n", IsTrusted(root))
	fmt.Fprintf(&b, "tools: list_directory, read_file, search_codebase, git_status, git_diff, git_log (auto)\n")
	fmt.Fprintf(&b, "tools: apply_edit (diff approval), run_command (bounded, approval), get_diagnostics (n/a)\n")
	fmt.Fprintf(&b, "run_command: no shell (argv only), metachars rejected, workspace-bounded cwd, 60s timeout, output capped, secret env stripped\n")
	fmt.Fprintf(&b, "project memory: SALAD.md / CLAUDE.md at workspace root\n")
	fmt.Fprintf(&b, "trust store: config-dir trusted_workspaces.json (legacy .salad-trust still honored)\n")
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

func trimOutput(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… truncated …\n"
}
