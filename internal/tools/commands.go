package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Classification decides whether a parsed command can run unattended.
type Classification int

const (
	ClassificationDenied Classification = iota
	ClassificationAuto
	ClassificationApproval
)

// readOnlyGit subcommands are safe to run without approval because they never
// write to the repository, network, or index. Anything else from git requires
// approval (branch -d, add, commit, checkout, reset, push, pull, clean...).
var readOnlyGit = map[string]bool{
	"status":        true,
	"diff":          true,
	"log":           true,
	"show":          true,
	"ls-files":      true,
	"ls-tree":       true,
	"rev-parse":     true,
	"shortlog":      true,
	"describe":      true,
	"blame":         true,
	"grep":          true,
	"fsck":          true,
	"check-ignore":  true,
	"count-objects": true,
	"for-each-ref":  true,
}

// ParseCommand splits a command string into argv without invoking a shell.
// It understands single/double quotes and backslash escapes, and rejects shell
// metacharacters (; | & < > ` and $( ) and unterminated quotes.
func ParseCommand(cmd string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	var quote rune
	started := false
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch quote {
		case '\'':
			if c == '\'' {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
			continue
		case '"':
			if c == '"' {
				quote = 0
				continue
			}
			if c == '\\' && i+1 < len(cmd) {
				i++
				cur.WriteByte(cmd[i])
				continue
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\\':
			if i+1 < len(cmd) {
				i++
				cur.WriteByte(cmd[i])
				started = true
			}
		case '\'', '"':
			quote = rune(c)
			started = true
		case ' ', '\t', '\n', '\r':
			if started {
				tokens = append(tokens, cur.String())
				cur.Reset()
				started = false
			}
		case ';', '|', '&', '<', '>', '`':
			return nil, fmt.Errorf("shell metacharacter %q is not allowed in Salad Terminal commands", string(c))
		case '$':
			if i+1 < len(cmd) && cmd[i+1] == '(' {
				return nil, fmt.Errorf("command substitution is not allowed in Salad Terminal commands")
			}
			cur.WriteByte(c)
			started = true
		default:
			cur.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	if started {
		tokens = append(tokens, cur.String())
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return tokens, nil
}

// Classify returns how a parsed command should be handled plus a short reason
// for the approval panel. Denied commands must not run. Auto commands run
// without prompting. Everything else requires explicit user approval.
func Classify(argv []string) (Classification, string) {
	if len(argv) == 0 {
		return ClassificationDenied, "empty command"
	}
	bin := strings.ToLower(filepath.Base(argv[0]))
	switch bin {
	case "git":
		if len(argv) < 2 {
			return ClassificationApproval, "bare git needs a subcommand"
		}
		sub := strings.ToLower(argv[1])
		if readOnlyGit[sub] {
			return ClassificationAuto, "read-only git " + sub
		}
		return ClassificationApproval, "git " + sub + " can modify the repository"
	case "ls", "pwd", "echo", "which", "uname", "whoami", "printf", "true", "false":
		return ClassificationAuto, "read-only utility"
	case "go":
		if len(argv) >= 2 {
			switch argv[1] {
			case "version", "env":
				return ClassificationAuto, "read-only go " + argv[1]
			}
		}
		return ClassificationApproval, "go may build, test, or mutate the workspace"
	case "node", "npm", "pnpm", "yarn", "bun", "deno", "python", "python3",
		"cargo", "make", "gradle", "mvn", "dotnet", "ruby", "php", "docker":
		if len(argv) == 2 && argv[1] == "--version" {
			return ClassificationAuto, "version check"
		}
		return ClassificationApproval, bin + " may build, test, or mutate the workspace"
	default:
		return ClassificationApproval, "unrecognized command — runs locally with your approval"
	}
}

// Canonical returns a stable command key for session-scoped auto-approve.
func Canonical(argv []string) string {
	return strings.Join(argv, " ")
}

// CanonicalForRequest returns the canonical form of a run_command request.
func CanonicalForRequest(req Request) string {
	cmd, _ := req.Arguments["command"].(string)
	argv, err := ParseCommand(cmd)
	if err != nil {
		return ""
	}
	return Canonical(argv)
}
