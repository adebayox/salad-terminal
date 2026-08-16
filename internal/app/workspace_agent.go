package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/harness"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

type workspaceSessionMsg struct {
	generation uint64
	session    *harness.Session
	cleanup    func()
	err        error
}

type workspaceEventMsg struct {
	generation uint64
	event      harness.SessionEvent
	closed     bool
}

type workspacePromptMsg struct {
	generation uint64
	err        error
}

func startWorkspaceSessionCmd(client *api.Client, root string, generation uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return workspaceSessionMsg{generation: generation, err: fmt.Errorf("Salad sign-in is required before using the workspace agent")}
		}
		if !workspace.IsTrusted(root) {
			return workspaceSessionMsg{generation: generation, err: fmt.Errorf("workspace is not trusted; use /trust before starting workspace work")}
		}
		command := strings.TrimSpace(os.Getenv("SALAD_DSH_COMMAND"))
		if command == "" {
			command = harness.InstalledCommand()
		}
		if strings.TrimSpace(os.Getenv("SALAD_DSH_COMMAND")) == "" && command == harness.InstalledCommand() {
			_, _, _, installed, installErr := harness.InstallationStatus()
			if installErr != nil {
				return workspaceSessionMsg{generation: generation, err: fmt.Errorf("managed Salad Harness install is invalid: %w", installErr)}
			}
			if !installed {
				return workspaceSessionMsg{generation: generation, err: fmt.Errorf("the workspace agent is not installed; update Salad Terminal to install its harness runtime")}
			}
		}

		configPath := firstNonEmpty(harness.InstalledConfig(), os.Getenv("SALAD_DSH_CONFIG"), os.Getenv("DSH_CORDIS_CONFIG"))
		opts := harness.Options{Command: command, Cwd: root}
		if configPath != "" {
			opts.Args = []string{"--config", configPath}
		}
		provider := strings.TrimSpace(os.Getenv("SALAD_HARNESS_PROVIDER"))
		var proxy *harness.ProviderProxy
		opts.Provider = provider
		if strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")) == "" && command == harness.InstalledCommand() {
			var err error
			proxy, err = harness.StartProviderProxy(context.Background(), client, provider)
			if err != nil {
				return workspaceSessionMsg{generation: generation, err: err}
			}
			opts.Env = append(opts.Env, proxy.Environment()...)
		}
		session, err := harness.StartSession(context.Background(), opts)
		if err != nil {
			if proxy != nil {
				_ = proxy.Close(context.Background())
			}
			return workspaceSessionMsg{generation: generation, err: err}
		}
		cleanup := func() {
			if proxy != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = proxy.Close(ctx)
			}
		}
		return workspaceSessionMsg{generation: generation, session: session, cleanup: cleanup}
	}
}

func waitWorkspaceEvent(events <-chan harness.SessionEvent, generation uint64) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		return workspaceEventMsg{generation: generation, event: event, closed: !ok}
	}
}

func sendWorkspacePromptCmd(session *harness.Session, prompt string, generation uint64) tea.Cmd {
	return func() tea.Msg {
		if session == nil {
			return workspacePromptMsg{generation: generation, err: fmt.Errorf("workspace agent is not ready")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		return workspacePromptMsg{generation: generation, err: session.Prompt(ctx, prompt)}
	}
}

func humanizeWorkspaceError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "http 401") || strings.Contains(text, "unauthorized") || strings.Contains(text, "sign-in has expired") {
		return "Salad sign-in has expired. Run `salad login`, then retry the workspace request."
	}
	if strings.Contains(text, "http 403") || strings.Contains(text, "provider_key_missing") || strings.Contains(text, "model_not_allowed") {
		return "This Salad model is not available for the terminal yet. Choose a configured provider/model in Salad, then retry."
	}
	if strings.Contains(text, "http 429") || strings.Contains(text, "quota") || strings.Contains(text, "rate limit") {
		return "The Salad terminal model limit has been reached. Try again after the limit resets, or use your own configured provider key."
	}
	if strings.Contains(text, "http 413") || strings.Contains(text, "request too large") {
		return "This workspace request is too large. Ask Salad to work on one file or one smaller task at a time."
	}
	if strings.Contains(text, "provider") || strings.Contains(text, "http 502") || strings.Contains(text, "http 503") {
		return "Salad's workspace model provider is unavailable. Retry with `salad --salad-provider <configured-provider>` or set SALAD_HARNESS_PROVIDER."
	}
	humanized := api.HumanizeError(err)
	if humanized == "Salad is temporarily unavailable. Try again in a moment." {
		return "Salad's workspace model provider is unavailable. Retry with `salad --salad-provider <configured-provider>` or set SALAD_HARNESS_PROVIDER."
	}
	return humanized
}

func (m *model) closeWorkspace() {
	m.workspaceGeneration++
	if m.workspaceSession != nil {
		m.workspaceSession.Close()
		m.workspaceSession = nil
	}
	if m.workspaceCleanup != nil {
		m.workspaceCleanup()
		m.workspaceCleanup = nil
	}
}

func (m *model) appendWorkspaceAssistant(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if len(m.messages) > 0 {
		last := &m.messages[len(m.messages)-1]
		if last.Role == "assistant" && last.AuthorName == "Salad" {
			last.Body += text
			m.refreshViewport()
			m.viewport.GotoBottom()
			return
		}
	}
	m.messages = append(m.messages, api.ChatMessage{Role: "assistant", AuthorName: "Salad", Body: text, CreatedAt: time.Now()})
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *model) appendWorkspaceActivity(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if len(text) > 2400 {
		text = text[:2400] + "…"
	}
	m.messages = append(m.messages, api.ChatMessage{Role: "system", AuthorName: "local", Body: text, CreatedAt: time.Now()})
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func workspaceToolLabel(event harness.SessionEvent) string {
	name := strings.TrimSpace(event.ToolName)
	if name == "" {
		name = "workspace tool"
	}
	switch event.ToolKind {
	case "read", "search":
		return "Looking through the project with " + name
	case "edit":
		return "Preparing a project edit with " + name
	case "execute":
		return "Running a project check with " + name
	case "fetch":
		return "Fetching information with " + name
	default:
		return "Using " + name + " in the project"
	}
}

func workspaceEventText(event harness.SessionEvent) string {
	switch event.Kind {
	case "tool_start":
		text := workspaceToolLabel(event)
		if event.ToolInput != "" {
			text += "\n" + event.ToolInput
		}
		return text
	case "tool_end":
		status := "finished"
		if strings.EqualFold(event.ToolStatus, "failed") {
			status = "could not finish"
		}
		text := "Workspace action " + status
		if event.ToolOutput != "" {
			text += "\n" + event.ToolOutput
		}
		return text
	case "plan":
		var lines []string
		for _, item := range event.Plan {
			mark := "•"
			if item.Status == "completed" {
				mark = "✓"
			} else if item.Status == "in_progress" {
				mark = "→"
			}
			lines = append(lines, mark+" "+item.Content)
		}
		if len(lines) == 0 {
			return "Workspace plan updated"
		}
		return "Workspace plan\n" + strings.Join(lines, "\n")
	default:
		return ""
	}
}
