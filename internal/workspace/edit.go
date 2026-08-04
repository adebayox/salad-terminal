package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxEditFileBytes = 512 * 1024
	commandTimeout   = 60 * time.Second
	maxCommandOutput = 32 * 1024
	maxDiffOutput    = 32 * 1024
)

// UnifiedDiff returns a unified-style diff between oldText and newText for a
// workspace-relative path. It trims common prefix/suffix lines and renders a
// single middle hunk with up to 3 lines of context. Designed for full-file
// apply_edit results, not whole-repo patch application.
func UnifiedDiff(path, oldText, newText string) (string, error) {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	midOld := oldLines[prefix : len(oldLines)-suffix]
	midNew := newLines[prefix : len(newLines)-suffix]

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n", path)
	fmt.Fprintf(&b, "+++ b/%s\n", path)

	if len(midOld) == 0 && len(midNew) == 0 {
		b.WriteString("(no changes)\n")
		return b.String(), nil
	}

	const contextLines = 3
	beforeStart := prefix - contextLines
	if beforeStart < 0 {
		beforeStart = 0
	}
	afterEnd := len(oldLines) - suffix + contextLines
	if afterEnd > len(oldLines) {
		afterEnd = len(oldLines)
	}

	oldStart := beforeStart + 1
	oldCount := len(midOld) + (prefix - beforeStart) + (afterEnd - (len(oldLines) - suffix))
	newStart := beforeStart + 1
	newCount := len(midNew) + (prefix - beforeStart) + (afterEnd - (len(oldLines) - suffix))

	if len(oldLines) == 0 {
		oldStart, oldCount = 0, 0
	}
	if len(newLines) == 0 {
		newStart, newCount = 0, 0
	}

	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)

	totalLines := oldCount + newCount
	if totalLines > 4000 {
		b.WriteString("(change is large; showing head and tail)\n")
		writeHunkLines(&b, oldLines[beforeStart:prefix], " ")
		writeHunkLines(&b, midOld, "-")
		writeHunkLines(&b, midNew, "+")
		writeHunkLines(&b, oldLines[len(oldLines)-suffix:afterEnd], " ")
		return trimOutput(b.String(), maxDiffOutput), nil
	}

	writeHunkLines(&b, oldLines[beforeStart:prefix], " ")
	writeHunkLines(&b, midOld, "-")
	writeHunkLines(&b, midNew, "+")
	writeHunkLines(&b, oldLines[len(oldLines)-suffix:afterEnd], " ")
	return trimOutput(b.String(), maxDiffOutput), nil
}

func writeHunkLines(b *strings.Builder, lines []string, prefix string) {
	for _, line := range lines {
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// resolveWriteTarget returns the real filesystem path for a workspace-relative
// path that may not exist yet. Existing paths are symlink-resolved and
// boundary-checked; missing paths resolve against the nearest existing ancestor
// so new files can be created inside the workspace without following symlinks
// outside it.
func resolveWriteTarget(root, relPath string) (abs string, rel string, err error) {
	root, err = RequireTrusted(root)
	if err != nil {
		return "", "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	relPath = filepath.Clean(relPath)
	if strings.HasPrefix(relPath, "..") {
		return "", "", fmt.Errorf("path escapes workspace")
	}
	candidate := resolvedRoot
	if relPath != "" && relPath != "." {
		candidate = filepath.Join(resolvedRoot, relPath)
	}
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if !isInsideRoot(resolvedRoot, resolved) {
			return "", "", fmt.Errorf("path escapes workspace")
		}
		rel, err = filepath.Rel(resolvedRoot, resolved)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", "", fmt.Errorf("path escapes workspace")
		}
		return resolved, filepath.ToSlash(rel), nil
	}
	parent := filepath.Dir(candidate)
	for {
		resolvedParent, parentErr := filepath.EvalSymlinks(parent)
		if parentErr == nil {
			if !isInsideRoot(resolvedRoot, resolvedParent) {
				return "", "", fmt.Errorf("path escapes workspace")
			}
			tail, _ := filepath.Rel(parent, candidate)
			target := filepath.Join(resolvedParent, tail)
			if !isInsideRoot(resolvedRoot, target) {
				return "", "", fmt.Errorf("path escapes workspace")
			}
			rel, err = filepath.Rel(resolvedRoot, target)
			if err != nil || strings.HasPrefix(rel, "..") {
				return "", "", fmt.Errorf("path escapes workspace")
			}
			return target, filepath.ToSlash(rel), nil
		}
		nextParent := filepath.Dir(parent)
		if nextParent == parent {
			break
		}
		parent = nextParent
	}
	return "", "", fmt.Errorf("path escapes workspace")
}

// PreviewEdit returns the unified diff of applying newContent to relPath
// without writing anything. Used for the approval panel.
func PreviewEdit(root, relPath, newContent string) (string, error) {
	if len(newContent) > maxEditFileBytes {
		return "", fmt.Errorf("new content too large (%d bytes > %d)", len(newContent), maxEditFileBytes)
	}
	abs, rel, err := resolveWriteTarget(root, relPath)
	if err != nil {
		return "", err
	}
	if isIgnored(root, rel, loadSaladIgnore(root)) {
		return "", fmt.Errorf("path blocked by .saladignore / defaults: %s", rel)
	}
	oldContent := ""
	if data, err := os.ReadFile(abs); err == nil {
		if len(data) > maxEditFileBytes {
			return "", fmt.Errorf("file too large to edit (%d bytes > %d)", len(data), maxEditFileBytes)
		}
		if !utf8.Valid(data) {
			return "", fmt.Errorf("file is not valid UTF-8 text: %s", rel)
		}
		oldContent = string(data)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return UnifiedDiff(rel, oldContent, newContent)
}

// ApplyEdit writes newContent to relPath (full-file replace) after safety
// checks, returning the applied diff. The write is atomic (temp file + rename),
// preserves the existing file mode, and creates missing parent directories
// inside the workspace for new files.
func ApplyEdit(root, relPath, newContent, description string) (string, error) {
	if len(newContent) > maxEditFileBytes {
		return "", fmt.Errorf("new content too large (%d bytes > %d)", len(newContent), maxEditFileBytes)
	}
	abs, rel, err := resolveWriteTarget(root, relPath)
	if err != nil {
		return "", err
	}
	if isIgnored(root, rel, loadSaladIgnore(root)) {
		return "", fmt.Errorf("path blocked by .saladignore / defaults: %s", rel)
	}

	mode := os.FileMode(0o644)
	oldContent := ""
	if data, err := os.ReadFile(abs); err == nil {
		if len(data) > maxEditFileBytes {
			return "", fmt.Errorf("file too large to edit (%d bytes > %d)", len(data), maxEditFileBytes)
		}
		if !utf8.Valid(data) {
			return "", fmt.Errorf("file is not valid UTF-8 text: %s", rel)
		}
		oldContent = string(data)
		if info, statErr := os.Stat(abs); statErr == nil {
			mode = info.Mode().Perm()
		}
	} else if !os.IsNotExist(err) {
		return "", err
	} else if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("cannot create parent directory for %s: %w", rel, err)
	}

	diff, err := UnifiedDiff(rel, oldContent, newContent)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), ".salad-edit-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return "", err
	}

	summary := "Applied edit to " + rel
	if strings.TrimSpace(description) != "" {
		summary += " (" + strings.TrimSpace(description) + ")"
	}
	return summary + "\n" + diff, nil
}

// RunCommand executes an already-approved argv directly (no shell) inside the
// workspace with a bounded timeout and capped output. It never interprets shell
// metacharacters; callers parse + classify before invoking.
func RunCommand(root string, argv []string, relCwd string) (string, error) {
	root, err := RequireTrusted(root)
	if err != nil {
		return "", err
	}
	if len(argv) == 0 {
		return "", fmt.Errorf("empty command")
	}
	dir := root
	if strings.TrimSpace(relCwd) != "" {
		abs, _, cwdErr := resolveInsideWorkspace(root, relCwd)
		if cwdErr != nil {
			return "", cwdErr
		}
		dir = abs
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = sanitizedEnv()
	out, err := cmd.CombinedOutput()
	output := trimOutput(string(out), maxCommandOutput)

	if ctx.Err() == context.DeadlineExceeded {
		return output + "\n… command timed out after " + commandTimeout.String() + " …\n",
			fmt.Errorf("command timed out after %s", commandTimeout)
	}
	if err != nil {
		code := -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return output + fmt.Sprintf("\n[exit %d]\n", code), fmt.Errorf("command failed (exit %d)", code)
	}
	return output, nil
}

// sanitizedEnv passes the user environment through but strips variables that
// look like credentials, so model-chosen commands cannot trivially read secrets.
func sanitizedEnv() []string {
	blocked := []string{"TOKEN", "SECRET", "KEY", "PASSWORD", "PRIVATE", "CREDENTIAL", "PASSWD"}
	var out []string
	for _, kv := range os.Environ() {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		upper := strings.ToUpper(name)
		skip := false
		for _, b := range blocked {
			if strings.Contains(upper, b) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, kv)
		}
	}
	return out
}

// ProjectMemory returns the contents of SALAD.md (then CLAUDE.md) at the
// workspace root, if present. Empty if neither exists or both are blocked.
func ProjectMemory(root string) (string, error) {
	root, err := ResolveRoot(root)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, name := range []string{"SALAD.md", "CLAUDE.md"} {
		content, readErr := ReadFile(root, name)
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		parts = append(parts, "["+name+"]\n"+strings.TrimSpace(content))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no project memory file (SALAD.md / CLAUDE.md)")
	}
	return strings.Join(parts, "\n\n"), nil
}
