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
		// Continue last workspace chat when known; otherwise resume picker.
		return tui.Run("")
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "--resume", "-r":
		return tui.RunResume()
	case "new":
		return tui.RunNew()
	case "login":
		baseURL := config.BaseURL()
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--google" || rest[i] == "google" {
				return auth.LoginGoogleBrowser(baseURL)
			}
			if rest[i] == "--base-url" && i+1 < len(rest) {
				baseURL = rest[i+1]
			}
		}
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
		if rest[0] == "pick" || rest[0] == "open" {
			return tui.RunResume()
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
		noTUI := false
		chatID := ""
		for _, arg := range rest {
			if arg == "--no-tui" {
				noTUI = true
				continue
			}
			if chatID == "" {
				chatID = arg
			}
		}
		if chatID == "" {
			return tui.RunResume()
		}
		if err := chat.Resume(chatID); err != nil {
			return err
		}
		if noTUI {
			return nil
		}
		return tui.Run(chatID)
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
		if _, err := workspace.EnsureTrusted(""); err != nil {
			return err
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
		if _, err := workspace.EnsureTrusted(""); err != nil {
			return err
		}
		out, err := workspace.GitStatus("")
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "git-diff", "diff":
		if _, err := workspace.EnsureTrusted(""); err != nil {
			return err
		}
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
	fmt.Print(`∬alad Terminal — same Salad, in your repo

  salad                 Continue last chat for this folder (or open resume picker)
  salad --resume        Pick a Salad chat (↑↓ · enter · n new · 1-9)
  salad new             Create a new Salad chat (syncs to web) and open it
  salad resume <id>     Jump straight into a chat
  salad login           Email/password sign-in
  salad login --google  Browser Google OAuth
  salad logout | whoami
  salad chat            List chats (headless)
  salad say <message>   Quick send to active chat
  salad workspace …     Local trust / read / git / permissions

In a chat: @ mention · /git /read · /new · /resume · esc = picker · ctrl+c quit
New chats are real Salad chats — they show up on the web app too.
Default API: staging (https://api-staging.salad.ink)
`)
}
