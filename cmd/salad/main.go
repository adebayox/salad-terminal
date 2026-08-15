package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/chat"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/harness"
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
	case "harness":
		if hasHelp(rest) {
			printCommandUsage("harness")
			return nil
		}
		return runHarness(rest)
	case "engineer":
		if hasHelp(rest) {
			printCommandUsage("engineer")
			return nil
		}
		return runEngineer(rest)
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

func runHarness(args []string) error {
	return runHarnessMode(args, false)
}

func runEngineer(args []string) error {
	return runHarnessMode(args, true)
}

func runHarnessMode(args []string, interactive bool) error {
	surface := "salad harness"
	if interactive {
		surface = "salad engineer"
	}
	if len(args) == 0 && !interactive {
		return fmt.Errorf("usage: salad harness [install|rollback|doctor|options] <prompt>")
	}
	if len(args) > 0 && args[0] == "install" {
		return runHarnessInstall(args[1:])
	}
	if len(args) > 0 && args[0] == "rollback" {
		if len(args) != 1 {
			return fmt.Errorf("usage: salad harness rollback")
		}
		runtimePath, err := harness.Rollback()
		if err != nil {
			return err
		}
		fmt.Printf("Restored previous Salad Harness runtime: %s\n", runtimePath)
		return nil
	}
	resumeOf := ""
	if len(args) > 0 && args[0] == "resume" {
		if len(args) < 2 {
			return fmt.Errorf("usage: salad harness resume <run-id> <prompt>")
		}
		record, err := harness.LoadRun(args[1])
		if err != nil {
			return err
		}
		root, err := workspace.ResolveRoot("")
		if err != nil || root != record.Workspace {
			return fmt.Errorf("resume must run from the original workspace: %s", record.Workspace)
		}
		resumeOf = record.ID
		args = append([]string{"--protocol", record.Protocol}, args[2:]...)
		if record.Command != "" {
			args = append([]string{"--command", record.Command}, args...)
		}
		if record.Config != "" {
			args = append([]string{"--config", record.Config}, args...)
		}
		if record.SessionID != "" {
			args = append([]string{"--session", record.SessionID}, args...)
		}
		if record.Protocol != "jsonrpc" {
			args = append([]string{"Continue the previous Salad Harness run. Previous request: " + record.Prompt + ". New request:"}, args...)
		}
	}
	if len(args) > 0 && args[0] == "doctor" {
		if len(args) != 1 {
			return fmt.Errorf("harness doctor does not take options; use `salad help harness`")
		}
		return runHarnessDoctor()
	}
	command, configPath, protocol, provider, model, sessionID, chatID := "", "", firstNonEmpty(os.Getenv("SALAD_DSH_PROTOCOL"), "acp"), "", "", "", ""
	networkMode := "deny"
	networkExplicit := false
	saladProvider := strings.TrimSpace(os.Getenv("SALAD_HARNESS_PROVIDER"))
	var prompt []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--command", "--config", "--protocol", "--provider", "--model", "--session", "--chat", "--salad-provider", "--network":
			if i+1 >= len(args) {
				return fmt.Errorf("harness option value is missing; use `salad harness --help`")
			}
			switch args[i] {
			case "--command":
				command = args[i+1]
			case "--config":
				configPath = args[i+1]
			case "--protocol":
				protocol = args[i+1]
			case "--provider":
				provider = args[i+1]
			case "--model":
				model = args[i+1]
			case "--session":
				sessionID = args[i+1]
			case "--chat":
				chatID = args[i+1]
			case "--salad-provider":
				saladProvider = args[i+1]
			case "--network":
				networkMode = args[i+1]
				networkExplicit = true
			}
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown harness option %q; use `salad harness --help`", args[i])
			}
			prompt = append(prompt, args[i])
		}
	}
	if protocol != "acp" && protocol != "jsonrpc" {
		return fmt.Errorf("unsupported harness protocol %q; choose acp or jsonrpc", protocol)
	}
	if interactive && protocol != "acp" {
		return errors.New("salad engineer uses the ACP session runtime; jsonrpc is available only through `salad harness`")
	}
	if networkMode != "deny" && networkMode != "allow" {
		return fmt.Errorf("unsupported harness network mode %q; choose deny or allow", networkMode)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DSH_NETWORK_MODE")), "allow") && !networkExplicit {
		return fmt.Errorf("network access is denied by default; pass `--network allow` to request it for this run")
	}
	if len(prompt) == 0 {
		if !interactive {
			return fmt.Errorf("harness prompt cannot be empty")
		}
	}
	root, err := workspace.EnsureTrusted("")
	if err != nil {
		return err
	}
	if err := requireInteractive(surface); err != nil {
		return err
	}
	input := io.Reader(os.Stdin)
	if networkMode == "allow" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Fprintln(os.Stderr, "Network access lets model-controlled commands contact the internet and may expose workspace data.")
		fmt.Fprint(os.Stderr, "Enable network for this run? [y/N] ")
		line, readErr := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if readErr != nil || (answer != "y" && answer != "yes") {
			return errors.New("network access was not enabled")
		}
		input = reader
	}
	if command == "" {
		command = harness.InstalledCommand()
	}
	opts := harness.Options{
		Command: command, Provider: provider, Model: model, SessionID: sessionID, Cwd: root,
		Input: input, InputCloser: os.Stdin, Output: os.Stdout,
	}
	if configPath == "" {
		configPath = firstNonEmpty(harness.InstalledConfig(), os.Getenv("SALAD_DSH_CONFIG"), os.Getenv("DSH_CORDIS_CONFIG"))
	}
	if protocol == "acp" && configPath != "" {
		opts.Args = []string{"--config", configPath}
	}
	if protocol == "jsonrpc" && configPath != "" {
		opts.Env = []string{"DSH_CORDIS_CONFIG=" + configPath}
	}
	if networkMode == "allow" {
		opts.Env = append(opts.Env, "DSH_NETWORK_MODE=allow")
	}
	var providerClient *api.Client
	var providerProxy *harness.ProviderProxy
	if strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")) == "" && command == harness.InstalledCommand() {
		var authErr error
		providerClient, _, authErr = auth.AuthedClient()
		if authErr != nil {
			return fmt.Errorf("the installed Salad Harness needs a signed-in Salad account or DEEPSEEK_API_KEY: %w", authErr)
		}
		providerProxy, err = harness.StartProviderProxy(context.Background(), providerClient, saladProvider)
		if err != nil {
			return err
		}
		defer func() {
			closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = providerProxy.Close(closeContext)
		}()
		opts.Env = append(opts.Env, providerProxy.Environment()...)
	}
	workspaceID, _ := workspace.OpaqueID(root)
	runPrefix := "salad-harness"
	if interactive {
		runPrefix = "salad-engineer"
	}
	runID := fmt.Sprintf("%s-%d", runPrefix, time.Now().UnixNano())
	if protocol == "jsonrpc" && sessionID == "" {
		sessionID = "salad-" + runID
	}
	if protocol == "jsonrpc" || protocol == "acp" {
		sessionRoot, sessionErr := harness.EnsureSessionRoot(root)
		if sessionErr != nil {
			return sessionErr
		}
		if !environmentValue(opts.Env, "DSH_SESSION_ROOT") && strings.TrimSpace(os.Getenv("DSH_SESSION_ROOT")) == "" {
			opts.Env = append(opts.Env, "DSH_SESSION_ROOT="+sessionRoot)
		}
		// The pinned ACP composition names its durable root
		// DSH_SNAPSHOT_SESSIONS_ROOT. Keep both names explicit so the managed
		// carrier and the JSON-RPC compatibility carrier share the same private,
		// per-workspace persistence boundary without inheriting a parent-shell
		// path.
		if !environmentValue(opts.Env, "DSH_SNAPSHOT_SESSIONS_ROOT") && strings.TrimSpace(os.Getenv("DSH_SNAPSHOT_SESSIONS_ROOT")) == "" {
			opts.Env = append(opts.Env, "DSH_SNAPSHOT_SESSIONS_ROOT="+sessionRoot)
		}
	}
	opts.SessionID = sessionID
	displayName := "harness"
	if interactive {
		displayName = "engineer"
	}
	fmt.Printf("[%s] run id: %s\n", displayName, runID)
	recordPrompt := strings.Join(prompt, " ")
	if recordPrompt == "" {
		recordPrompt = "(interactive engineer session)"
	}
	runRecord := harness.RunRecord{ID: runID, Workspace: root, Protocol: protocol, SessionID: sessionID, Command: command, Config: configPath, Prompt: recordPrompt, StartedAt: time.Now().UTC(), ResumeOf: resumeOf}
	if err := harness.SaveRun(runRecord); err != nil {
		return fmt.Errorf("save harness run record: %w", err)
	}
	if chatID == "" {
		chatID = strings.TrimSpace(os.Getenv("SALAD_HARNESS_CHAT_ID"))
	}
	if chatID == "" {
		if active, activeErr := config.LoadActiveChat(); activeErr == nil {
			chatID = active.ChatID
		}
	}
	postHarnessEvent(context.Background(), providerClient, chatID, runID, workspaceID, "started", "Harness run started")
	postHarnessEventWithSequence(context.Background(), providerClient, chatID, runID, workspaceID, "running", "Harness is working in the trusted workspace", 2)
	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var result harness.Result
	if protocol == "jsonrpc" {
		result, err = harness.Run(runContext, opts, strings.Join(prompt, " "))
	} else if interactive {
		result, err = harness.RunACPInteractive(runContext, opts, strings.Join(prompt, " "))
	} else {
		result, err = harness.RunACP(runContext, opts, strings.Join(prompt, " "))
	}
	if result.SessionID != "" && result.SessionID != runRecord.SessionID {
		runRecord.SessionID = result.SessionID
		if saveErr := harness.SaveRun(runRecord); saveErr != nil {
			fmt.Fprintf(os.Stderr, "[harness] could not save the actual session id: %v\n", saveErr)
		}
	}
	status, summary := "completed", "Harness run completed"
	if err != nil {
		status = "failed"
		summary = "Harness run failed"
		if errors.Is(err, context.Canceled) {
			status = "cancelled"
			summary = "Harness run cancelled"
		}
	}
	postHarnessEventWithSequence(context.Background(), providerClient, chatID, runID, workspaceID, status, summary, 3)
	return err
}

func environmentValue(values []string, name string) bool {
	for _, value := range values {
		if key, _, ok := strings.Cut(value, "="); ok && key == name {
			return true
		}
	}
	return false
}

func postHarnessEvent(ctx context.Context, client *api.Client, chatID, runID, workspaceID, status, summary string) {
	postHarnessEventWithSequence(ctx, client, chatID, runID, workspaceID, status, summary, 1)
}

func postHarnessEventWithSequence(ctx context.Context, client *api.Client, chatID, runID, workspaceID, status, summary string, sequence int64) {
	if strings.TrimSpace(chatID) == "" || strings.TrimSpace(workspaceID) == "" {
		return
	}
	if client == nil {
		var err error
		client, _, err = auth.AuthedClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[harness] Salad chat receipt unavailable: %v\n", err)
			return
		}
	}
	// Receipt posts may be the first authenticated request after a long-lived
	// terminal session. Give the client enough time to refresh an expired
	// access token on a slow edge, without allowing a stalled receipt to hold
	// the harness run indefinitely.
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.PostHarnessRunEvent(requestCtx, api.HarnessRunEventRequest{
		ChatID: chatID, RunID: runID, WorkspaceID: workspaceID, Status: status, Summary: summary, Sequence: sequence,
	}); err != nil {
		// Lifecycle receipts are optional collaboration affordances. A server
		// that has only the provider bridge intentionally returns 404 here; do
		// not make a successful local engineer run look broken or noisy.
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return
		}
		fmt.Fprintf(os.Stderr, "[harness] Salad chat receipt unavailable: %v\n", err)
	}
}

func runHarnessInstall(args []string) error {
	if len(args) == 0 || hasHelp(args) {
		printCommandUsage("harness")
		return nil
	}
	runtimePath, configPath := "", ""
	force := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runtime":
			if i+1 >= len(args) {
				return fmt.Errorf("harness install --runtime needs a path")
			}
			runtimePath = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return fmt.Errorf("harness install --config needs a path")
			}
			configPath = args[i+1]
			i++
		case "--force":
			force = true
		default:
			return fmt.Errorf("unknown harness install option %q", args[i])
		}
	}
	installed, err := harness.Install(runtimePath, configPath, force)
	if err != nil {
		return err
	}
	fmt.Printf("Installed Salad Harness runtime: %s\n", installed)
	if configPath != "" {
		fmt.Println("Installed project config for future runs.")
	}
	fmt.Println("Next: trust the project, then run `salad harness \"inspect this project and make a plan\"`.")
	return nil
}

func runHarnessDoctor() error {
	command := harness.InstalledCommand()
	protocol := firstNonEmpty(os.Getenv("SALAD_DSH_PROTOCOL"), "acp")
	configPath := firstNonEmpty(harness.InstalledConfig(), os.Getenv("DSH_CORDIS_CONFIG"))
	fmt.Println("Salad Harness doctor")
	fmt.Printf("protocol: %s\n", protocol)
	fmt.Printf("command: %s\n", command)
	executableAvailable := true
	if resolved, err := exec.LookPath(command); err == nil {
		fmt.Printf("executable: %s\n", resolved)
	} else {
		executableAvailable = false
		fmt.Printf("executable: missing (%s)\n", err)
	}
	if runtimePath, installedConfig, digest, installed, err := harness.InstallationStatus(); err == nil {
		if installed {
			fmt.Printf("installed runtime: %s\n", runtimePath)
			fmt.Printf("installed sha256: %s\n", digest)
			if installedConfig != "" {
				fmt.Printf("installed config: %s\n", installedConfig)
			}
		} else {
			fmt.Println("managed install: not installed")
		}
	} else {
		fmt.Printf("managed install: invalid (%s)\n", err)
	}
	if configPath == "" {
		fmt.Println("config: not set (ACP command will look for cordis.yml in the workspace)")
	} else if info, err := os.Stat(configPath); err != nil {
		fmt.Printf("config: missing (%s)\n", err)
	} else {
		fmt.Printf("config: %s (%s)\n", configPath, info.Mode().Type())
	}
	if !executableAvailable {
		return fmt.Errorf("Salad Harness executable is unavailable; install a platform carrier or set SALAD_DSH_COMMAND")
	}
	return nil
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
  salad engineer        Work with an agent in this trusted workspace

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
	case "harness":
		fmt.Print(`Usage: salad harness [install|rollback|doctor|options] <prompt>

Run a DeepSeek Harness session in the trusted current workspace. Install a
runtime is installed automatically by the macOS/Linux release installer.
"salad harness install" is for development or a manually supplied carrier;
"salad harness rollback" restores the last managed carrier. This does not use
or change your normal Salad chat. "salad harness doctor" checks the setup.

  install --runtime <path> [--config <path>] [--force]
  rollback                 Restore the previous managed runtime
  resume <run-id> <prompt>  Continue from a saved local run record

  --command <path>        DSH ACP executable (or SALAD_DSH_COMMAND)
  --config <path>         ACP cordis.yml (or SALAD_DSH_CONFIG)
  --protocol <name>       acp (default) or jsonrpc compatibility mode
  --provider <name>       DSH provider (or SALAD_DSH_PROVIDER)
  --model <name>          DSH model (or SALAD_DSH_MODEL)
  --network <mode>        deny (default) or allow, with a confirmation prompt
  --salad-provider <name> Salad provider for the authenticated local bridge
  --session <id>          Reuse a JSON-RPC session across runs
  --chat <id>             Share run start/finish with this Salad chat
`)
	case "engineer":
		fmt.Print(`Usage: salad engineer [prompt]

Work with an agent in the trusted current workspace. With no prompt, Salad
keeps one session open so you can inspect, edit, test, and follow up without
starting over. Press Ctrl-D to finish the session; Ctrl-C cancels the active
run and cleans up its child processes.

The normal Salad chat is a separate product path and is not used by this
command. Network access is denied by default. To request it for this run:

  salad engineer --network allow
`)
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
