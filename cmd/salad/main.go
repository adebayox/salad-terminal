package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/chat"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/theme"
	"github.com/salad-ai/salad-terminal/internal/tui"
	"github.com/salad-ai/salad-terminal/internal/update"
	"github.com/salad-ai/salad-terminal/internal/workspace"
	"golang.org/x/term"
)

// Version is stamped at build time (git short sha). See install.sh.
var Version = "dev"

const installURL = "https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh"
const installWindowsURL = "https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.ps1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", api.HumanizeError(err))
		if os.Getenv("SALAD_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "Details:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		if err := requireInteractive("salad"); err != nil {
			return err
		}
		if err := ensureLatest(); err != nil {
			return err
		}
		// Claude Code: bare launch = new session.
		return tui.Run("")
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "help", "-h", "--help":
		if len(rest) > 0 && rest[0] != "-h" && rest[0] != "--help" {
			printCommandUsage(rest[0])
		} else {
			printUsage()
		}
		return nil
	case "version", "--version", "-v":
		fmt.Println("salad", Version)
		return nil
	case "doctor":
		if hasHelp(rest) {
			printCommandUsage("doctor")
			return nil
		}
		return runDoctor()
	case "update":
		if hasHelp(rest) {
			printCommandUsage("update")
			return nil
		}
		return runUpdate()
	case "--continue", "-c", "continue":
		if hasHelp(rest) {
			printCommandUsage("continue")
			return nil
		}
		if err := requireInteractive("salad --continue"); err != nil {
			return err
		}
		if err := ensureLatest(); err != nil {
			return err
		}
		return tui.RunContinue()
	case "--resume", "-r":
		if hasHelp(rest) {
			printCommandUsage("resume")
			return nil
		}
		if err := requireInteractive("salad --resume"); err != nil {
			return err
		}
		if err := ensureLatest(); err != nil {
			return err
		}
		return tui.RunResume()
	case "new":
		if hasHelp(rest) {
			printCommandUsage("new")
			return nil
		}
		if err := requireInteractive("salad new"); err != nil {
			return err
		}
		if err := ensureLatest(); err != nil {
			return err
		}
		return tui.RunNew()
	case "login":
		if hasHelp(rest) {
			printCommandUsage("login")
			return nil
		}
		baseURL := config.BaseURL()
		email, password := "", ""
		google := false
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--email", "-e":
				if i+1 >= len(rest) {
					return fmt.Errorf("email is missing; use `salad login --help`")
				}
				email = rest[i+1]
				i++
			case "--password", "-p":
				if i+1 >= len(rest) {
					return fmt.Errorf("password is missing; use `salad login --help`")
				}
				password = rest[i+1]
				i++
			case "--base-url":
				if i+1 >= len(rest) {
					return fmt.Errorf("base URL is missing; use `salad login --help`")
				}
				baseURL = rest[i+1]
				i++
			case "--google", "google":
				google = true
			default:
				return fmt.Errorf("unknown login option %q; use `salad login --help`", rest[i])
			}
		}
		if google && (email != "" || password != "") {
			return fmt.Errorf("choose browser sign-in or email/password sign-in, not both; use `salad login --help`")
		}
		if google {
			return auth.LoginGoogleBrowser(baseURL)
		}
		if email != "" || password != "" {
			if email == "" || password == "" {
				return fmt.Errorf("email and password must be provided together; use `salad login --help`")
			}
			return auth.Login(baseURL, email, password)
		}
		return auth.LoginInteractive(baseURL)
	case "signup", "register":
		if hasHelp(rest) {
			printCommandUsage("signup")
			return nil
		}
		if len(rest) != 0 {
			return fmt.Errorf("signup does not take options; use `salad signup --help`")
		}
		return auth.OpenSignup()
	case "recover", "reset-password":
		if hasHelp(rest) {
			printCommandUsage("recover")
			return nil
		}
		if len(rest) != 0 {
			return fmt.Errorf("recover does not take options; use `salad recover --help`")
		}
		return auth.OpenRecovery()
	case "logout":
		if hasHelp(rest) {
			printCommandUsage("logout")
			return nil
		}
		if len(rest) != 0 {
			return fmt.Errorf("logout does not take options; use `salad logout --help`")
		}
		return auth.Logout()
	case "whoami":
		if hasHelp(rest) {
			printCommandUsage("whoami")
			return nil
		}
		if len(rest) != 0 {
			return fmt.Errorf("whoami does not take options; use `salad whoami --help`")
		}
		return auth.WhoAmI()
	case "chat", "chats":
		if hasHelp(rest) {
			printCommandUsage("chat")
			return nil
		}
		if len(rest) == 0 {
			return chat.List()
		}
		if rest[0] == "pick" || rest[0] == "open" {
			if err := requireInteractive("salad chat " + rest[0]); err != nil {
				return err
			}
			return tui.RunResume()
		}
		if rest[0] == "participants" {
			if len(rest) > 2 {
				return fmt.Errorf("usage: salad chat participants <chat-id>")
			}
			id := ""
			if len(rest) > 1 {
				id = rest[1]
			}
			return chat.ShowParticipants(id)
		}
		return fmt.Errorf("unknown chat subcommand %q; use `salad chat --help`", rest[0])
	case "resume":
		if hasHelp(rest) {
			printCommandUsage("resume")
			return nil
		}
		noTUI := false
		chatID := ""
		for _, arg := range rest {
			if arg == "--no-tui" {
				if noTUI {
					return fmt.Errorf("duplicate --no-tui; use `salad resume --help`")
				}
				noTUI = true
				continue
			}
			if chatID == "" {
				if strings.HasPrefix(arg, "-") {
					return fmt.Errorf("unknown resume option %q; use `salad resume --help`", arg)
				}
				chatID = arg
				continue
			}
			return fmt.Errorf("only one chat ID is allowed; use `salad resume --help`")
		}
		if chatID == "" {
			if err := requireInteractive("salad resume"); err != nil {
				return err
			}
			if err := ensureLatest(); err != nil {
				return err
			}
			return tui.RunResume()
		}
		if !noTUI {
			if err := requireInteractive("salad resume <chat-id>"); err != nil {
				return err
			}
		}
		if err := chat.Resume(chatID); err != nil {
			return err
		}
		if noTUI {
			return nil
		}
		if err := ensureLatest(); err != nil {
			return err
		}
		return tui.Run(chatID)
	case "say", "send":
		if hasHelp(rest) {
			printCommandUsage("say")
			return nil
		}
		if len(rest) < 1 {
			return fmt.Errorf("usage: salad say <message>")
		}
		if strings.TrimSpace(strings.Join(rest, " ")) == "" {
			return fmt.Errorf("message cannot be empty")
		}
		id, err := chat.ActiveChatID()
		if err != nil {
			return err
		}
		return chat.Send(id, strings.Join(rest, " "))
	case "workspace":
		if hasHelp(rest) {
			printCommandUsage("workspace")
			return nil
		}
		return runWorkspace(rest)
	default:
		return fmt.Errorf("unknown command %q; run `salad --help` to see available commands", cmd)
	}
}

func requireInteractive(command string) error {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	return fmt.Errorf("%s needs an interactive terminal; use `--help` for non-interactive commands", command)
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func ensureLatest() error {
	if !update.MaybeAutoUpdate(Version) {
		return nil
	}
	return update.Reexec()
}

func runUpdate() error {
	os.Setenv("SALAD_SKIP_AUTOUPDATE", "")
	fmt.Println("Updating Salad Terminal from GitHub…")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		command := "$env:SALAD_FORCE_REMOTE='1'; iex (Invoke-RestMethod -Uri '" + installWindowsURL + "')"
		// Windows keeps the running executable locked. Let the installer start
		// after this process exits instead of asking it to overwrite itself.
		command = "Start-Sleep -Seconds 1; " + command
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("could not start the updater: %w", err)
		}
		fmt.Println("Update started. The new version will be ready when you run salad again.")
		return nil
	} else {
		cmd = exec.Command("bash", "-c", "curl -fsSL "+installURL+" | SALAD_FORCE_REMOTE=1 bash")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Println("Done. Run salad again (or it will auto-update next launch).")
	return nil
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
		if len(args) > 2 {
			return fmt.Errorf("usage: salad workspace trust [path]")
		}
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
		if len(args) != 2 {
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
		if len(args) != 1 {
			return fmt.Errorf("usage: salad workspace git-status")
		}
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
		if len(args) != 1 {
			return fmt.Errorf("usage: salad workspace git-diff")
		}
		if _, err := workspace.EnsureTrusted(""); err != nil {
			return err
		}
		out, err := workspace.GitDiff("", "", true)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "permissions":
		if len(args) != 1 {
			return fmt.Errorf("usage: salad workspace permissions")
		}
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
	fmt.Println(theme.Banner(72))
	fmt.Printf(`
Same Salad chats, in your repo  (%s)

Start:
  salad                 Sign in if needed, then start a chat
  salad --continue      Continue this folder's chat
  salad --resume        Choose a previous chat

Account:
  salad login           Sign in from the terminal or browser
  salad signup          Create an account in your browser
  salad recover         Reset a forgotten password in your browser
  salad whoami          Show the signed-in account
  salad logout          Sign out on this computer

Chats:
  salad chat            List chats
  salad resume <id>     Open one chat
  salad say <message>   Send one message

Workspace:
  salad workspace ...   Trust, inspect, or check the current repo

Other:
  salad update          Install the latest release
  salad version         Show the installed version
  salad doctor          Check sign-in, API, and workspace setup

Run salad <command> --help for command details.
`, Version)
}

func printCommandUsage(command string) {
	switch command {
	case "login":
		fmt.Print(`Usage: salad login [options]

Sign in to Salad. With no options, choose email or browser sign-in.

  --google                 Sign in in your browser
  --email <address>        Use email/password sign-in
  --password <password>    Password for non-interactive use
  --base-url <url>         Use another Salad environment

Examples:
  salad login
  salad login --google
  salad login --email you@example.com --password '…'
`)
	case "signup":
		fmt.Println("Usage: salad signup")
		fmt.Println("Opens Salad account creation in your browser.")
	case "recover":
		fmt.Println("Usage: salad recover")
		fmt.Println("Opens Salad password recovery in your browser.")
	case "chat":
		fmt.Print(`Usage: salad chat [participants <chat-id>]

List your chats, or show the members in one chat.
`)
	case "say":
		fmt.Print(`Usage: salad say <message>

Send a message to the active chat. Use salad resume <chat-id> first if needed.
`)
	case "workspace":
		fmt.Print(`Usage: salad workspace <command>

  trust [path]             Allow local workspace tools for a repository
  permissions              Show local tool permissions
  read <path>              Read a non-sensitive file
  git-status               Show git status
  git-diff                 Show the current diff
`)
	case "continue":
		fmt.Println("Usage: salad --continue")
		fmt.Println("Continue the chat linked to the current repository.")
	case "resume":
		fmt.Print(`Usage: salad resume [chat-id] [--no-tui]

Open a chat by ID. Without an ID, choose from previous chats.
`)
	case "new":
		fmt.Println("Usage: salad new")
		fmt.Println("Start a new chat.")
	case "update":
		fmt.Println("Usage: salad update")
		fmt.Println("Install the latest published Salad Terminal release.")
	case "whoami":
		fmt.Println("Usage: salad whoami")
		fmt.Println("Show the account currently signed in on this computer.")
	case "logout":
		fmt.Println("Usage: salad logout")
		fmt.Println("Sign out on this computer and remove local Salad Terminal credentials.")
	case "doctor":
		fmt.Println("Usage: salad doctor")
		fmt.Println("Check the local install, sign-in, Salad API, and current workspace.")
	default:
		printUsage()
	}
}

func runDoctor() error {
	configDir, configErr := config.Dir()
	root, rootErr := workspace.ResolveRoot("")
	fmt.Println("Salad Terminal doctor")
	fmt.Printf("version: %s\n", Version)
	fmt.Printf("platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("api: %s\n", config.BaseURL())
	if configErr != nil {
		fmt.Printf("config: unavailable (%s)\n", api.HumanizeError(configErr))
	} else {
		fmt.Printf("config: %s\n", configDir)
	}
	if rootErr != nil {
		fmt.Printf("workspace: unavailable (%s)\n", api.HumanizeError(rootErr))
	} else {
		fmt.Printf("workspace: %s\n", root)
		fmt.Printf("workspace trust: %v\n", workspace.IsTrusted(root))
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := api.New(config.BaseURL(), "").Probe(probeCtx); err != nil {
		fmt.Printf("api reachability: failed — %s\n", api.HumanizeError(err))
	} else {
		fmt.Println("api reachability: ok")
	}

	client, creds, err := auth.AuthedClient()
	if err != nil {
		fmt.Println("sign-in: not signed in")
		fmt.Println("next: run salad to sign in")
		return nil
	}
	userCtx, userCancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer userCancel()
	user, userErr := client.Me(userCtx)
	if userErr != nil {
		fmt.Printf("sign-in: could not verify (%s)\n", api.HumanizeError(userErr))
		return nil
	}
	fmt.Printf("sign-in: %s\n", firstNonEmpty(user.Email, creds.Email, "signed in"))
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
