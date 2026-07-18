package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/chat"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/tui"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return tui.Run("")
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "login":
		baseURL := config.BaseURL()
		if len(rest) >= 2 && rest[0] == "--email" {
			email := rest[1]
			password := ""
			for i := 2; i+1 < len(rest); i += 2 {
				if rest[i] == "--password" {
					password = rest[i+1]
				}
				if rest[i] == "--base-url" {
					baseURL = rest[i+1]
				}
			}
			if password == "" {
				return fmt.Errorf("password required with --email")
			}
			return auth.Login(baseURL, email, password)
		}
		for i := 0; i+1 < len(rest); i += 2 {
			if rest[i] == "--base-url" {
				baseURL = rest[i+1]
			}
		}
		return auth.LoginInteractive(baseURL)
	case "logout":
		return auth.Logout()
	case "whoami":
		return auth.WhoAmI()
	case "chat", "chats":
		if len(rest) == 0 {
			return chat.List()
		}
		if rest[0] == "participants" {
			id := ""
			if len(rest) > 1 {
				id = rest[1]
			}
			return chat.ShowParticipants(id)
		}
		return fmt.Errorf("unknown chat subcommand %q", rest[0])
	case "resume":
		if len(rest) < 1 {
			return fmt.Errorf("usage: salad resume <chat-id>")
		}
		if err := chat.Resume(rest[0]); err != nil {
			return err
		}
		return tui.Run(rest[0])
	case "say", "send":
		if len(rest) < 1 {
			return fmt.Errorf("usage: salad say <message>")
		}
		id, err := chat.ActiveChatID()
		if err != nil {
			return err
		}
		return chat.Send(id, strings.Join(rest, " "))
	case "workspace":
		return runWorkspace(rest)
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func runWorkspace(args []string) error {
	if len(args) == 0 {
		summary, err := workspace.PermissionsSummary("")
		if err != nil {
			return err
		}
		fmt.Print(summary)
		return nil
	}
	switch args[0] {
	case "trust":
		root := ""
		if len(args) > 1 {
			root = args[1]
		}
		resolved, err := workspace.ResolveRoot(root)
		if err != nil {
			return err
		}
		if err := workspace.Trust(resolved); err != nil {
			return err
		}
		fmt.Println("Trusted", resolved)
		return nil
	case "read":
		if len(args) < 2 {
			return fmt.Errorf("usage: salad workspace read <path>")
		}
		content, err := workspace.ReadFile("", args[1])
		if err != nil {
			return err
		}
		fmt.Print(content)
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
		return nil
	case "git-status", "status":
		out, err := workspace.GitStatus("")
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "git-diff", "diff":
		out, err := workspace.GitDiff("")
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "permissions":
		summary, err := workspace.PermissionsSummary("")
		if err != nil {
			return err
		}
		fmt.Print(summary)
		return nil
	default:
		return fmt.Errorf("usage: salad workspace [trust|read|git-status|git-diff|permissions]")
	}
}

func printUsage() {
	fmt.Print(`Salad Terminal — equal Salad surface (CLI)

Usage:
  salad                      Open minimal TUI for the active chat
  salad login [--base-url U] Log in with normal Salad user credentials
  salad logout
  salad whoami
  salad chat                 List chats
  salad chat participants    Show participants for active chat
  salad resume <chat-id>     Resume chat and open TUI
  salad say <message>        Send to active chat
  salad workspace …          Local trust / read / git / permissions

Environment:
  SALAD_API_URL     API base (default https://api.salad.chat; use staging URL locally)
  SALAD_CONFIG_DIR  Override credentials directory

Contract: docs/TERMINAL_CONTRACT.md
`)
}
