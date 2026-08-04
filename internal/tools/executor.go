package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/salad-ai/salad-terminal/internal/workspace"
)

// Request is the salad.v1 tool_request payload.
type Request struct {
	RequestID   string         `json:"request_id"`
	ToolCallID  string         `json:"tool_call_id"`
	ToolName    string         `json:"tool_name"`
	Arguments   map[string]any `json:"arguments"`
	WorkspaceID string         `json:"workspace_id"`
}

// Execute runs a workspace tool against root. Read-only tools run directly;
// apply_edit and run_command are only reached through the app's approval gate
// (defense-in-depth re-parses/re-classifies run_command here too).
func Execute(root string, req Request) (string, error) {
	name := strings.TrimSpace(req.ToolName)
	args := req.Arguments
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "list_directory":
		path, _ := args["path"].(string)
		if strings.TrimSpace(path) == "" {
			path = "."
		}
		return workspace.ListDirectory(root, path)
	case "read_file":
		path, _ := args["path"].(string)
		if strings.TrimSpace(path) == "" {
			return "", fmt.Errorf("path required")
		}
		return workspace.ReadFile(root, path)
	case "search_codebase":
		query, _ := args["query"].(string)
		pattern, _ := args["file_pattern"].(string)
		return workspace.SearchCodebase(root, query, pattern)
	case "git_status":
		return workspace.GitStatus(root)
	case "git_diff":
		path, _ := args["path"].(string)
		statOnly := true
		if v, ok := args["stat_only"].(bool); ok {
			statOnly = v
		}
		return workspace.GitDiff(root, path, statOnly)
	case "git_log":
		limit := 10
		switch v := args["limit"].(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case json.Number:
			if n, err := v.Int64(); err == nil {
				limit = int(n)
			}
		}
		return workspace.GitLog(root, limit)
	case "get_diagnostics":
		return "Diagnostics are not available in Salad Terminal. Verify with run_command instead (e.g. 'go test ./...', 'go vet ./...', 'npm run build').", nil
	case "apply_edit":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		description, _ := args["description"].(string)
		if strings.TrimSpace(path) == "" {
			return "", fmt.Errorf("path required")
		}
		return workspace.ApplyEdit(root, path, content, description)
	case "run_command":
		command, _ := args["command"].(string)
		cwd, _ := args["cwd"].(string)
		argv, err := ParseCommand(command)
		if err != nil {
			return "", err
		}
		cls, reason := Classify(argv)
		if cls == ClassificationDenied {
			return "", fmt.Errorf("command not allowed: %s", reason)
		}
		return workspace.RunCommand(root, argv, cwd)
	default:
		return "", fmt.Errorf("unknown workspace tool %q", name)
	}
}

// PreviewEdit returns a human-readable diff for the approval panel without
// writing anything.
func PreviewEdit(root string, req Request) (string, error) {
	path, _ := req.Arguments["path"].(string)
	content, _ := req.Arguments["content"].(string)
	description, _ := req.Arguments["description"].(string)
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path required")
	}
	diff, err := workspace.PreviewEdit(root, path, content)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("file: " + path + "\n")
	if strings.TrimSpace(description) != "" {
		b.WriteString("why:  " + strings.TrimSpace(description) + "\n")
	}
	b.WriteString(diff)
	return b.String(), nil
}
