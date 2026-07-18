package bridge

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

// BuildCodeContext gathers turn-scoped local workspace context for terminal-initiated sends.
// Only runs when the workspace is trusted. Never includes ignored/secret paths.
func BuildCodeContext(root string, focusFiles []string) (*api.CodeContext, string, error) {
	root, err := workspace.ResolveRoot(root)
	if err != nil {
		return nil, "", err
	}
	if !workspace.IsTrusted(root) {
		return nil, "", fmt.Errorf("workspace not trusted (run: salad workspace trust)")
	}

	status, _ := workspace.GitStatus(root)
	diff, _ := workspace.GitDiff(root)

	ctx := &api.CodeContext{
		WorkspaceRoot: root,
		Language:      "multi",
	}
	var open []api.OpenFileContent
	for _, rel := range focusFiles {
		rel = filepath.Clean(rel)
		content, err := workspace.ReadFile(root, rel)
		if err != nil {
			continue
		}
		open = append(open, api.OpenFileContent{
			Path:      rel,
			Content:   content,
			Language:  guessLang(rel),
			LineCount: strings.Count(content, "\n") + 1,
		})
		ctx.OpenFiles = append(ctx.OpenFiles, rel)
	}
	ctx.OpenFilesContent = open

	summary := strings.Builder{}
	summary.WriteString("Terminal workspace context\n")
	summary.WriteString("root: " + root + "\n")
	if strings.TrimSpace(status) != "" {
		summary.WriteString("\ngit status:\n" + trim(status, 4000))
		ctx.Diagnostics = append(ctx.Diagnostics, "git_status:\n"+trim(status, 2000))
	}
	if strings.TrimSpace(diff) != "" {
		summary.WriteString("\ngit diff --stat:\n" + trim(diff, 4000))
		ctx.SelectedCode = trim(diff, 12000)
		ctx.SelectedFile = "git-diff"
	}
	return ctx, summary.String(), nil
}

func AttachSnippet(ctx *api.CodeContext, path, content string) *api.CodeContext {
	if ctx == nil {
		ctx = &api.CodeContext{}
	}
	ctx.SelectedFile = path
	ctx.SelectedCode = content
	ctx.Language = guessLang(path)
	ctx.OpenFiles = appendUnique(ctx.OpenFiles, path)
	ctx.OpenFilesContent = append(ctx.OpenFilesContent, api.OpenFileContent{
		Path:      path,
		Content:   content,
		Language:  guessLang(path),
		LineCount: strings.Count(content, "\n") + 1,
	})
	return ctx
}

func guessLang(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	default:
		return ""
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… truncated …\n"
}

func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}
