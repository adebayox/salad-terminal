package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/theme"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

type screen int

const (
	screenBoot screen = iota
	screenLogin
	screenChats
	screenRoom
)

type member struct {
	ID          string
	DisplayName string
	Slug        string
	MemberType  string
	Role        string
}

type model struct {
	width  int
	height int
	screen screen

	client *api.Client
	creds  *config.Credentials
	status string
	err    string

	// login
	loginEmail string
	loginPass  string
	loginFocus int // 0 email 1 pass

	// chats
	chats    []api.ChatPreview
	chatIdx  int
	chatLoad bool

	// room
	chatID       string
	chatTitle    string
	members      []member
	messages     []api.ChatMessage
	viewport     viewport.Model
	composer     textarea.Model
	sending      bool
	workspaceOK  bool
	workspaceDir string
}

type (
	bootMsg struct {
		client *api.Client
		creds  *config.Credentials
		err    error
	}
	chatsMsg struct {
		chats []api.ChatPreview
		err   error
	}
	roomMsg struct {
		title    string
		messages []api.ChatMessage
		members  []member
		err      error
	}
	pollMsg struct {
		messages []api.ChatMessage
		err      error
	}
	sentMsg struct {
		msg *api.ChatMessage
		err error
	}
	loginDoneMsg struct {
		err error
	}
	tickMsg time.Time
)

func Run(initialChatID string) error {
	m := newModel(initialChatID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(initialChatID string) model {
	ta := textarea.New()
	ta.Placeholder = "Message this Salad chat…  (@name to mention)"
	ta.ShowLineNumbers = false
	ta.Prompt = "∬ "
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	vp := viewport.New(80, 20)
	root, _ := workspace.ResolveRoot("")
	return model{
		screen:       screenBoot,
		status:       "Opening Salad…",
		composer:     ta,
		viewport:     vp,
		chatID:       initialChatID,
		workspaceDir: root,
		workspaceOK:  workspace.IsTrusted(root),
		loginFocus:   0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(bootCmd(), textarea.Blink)
}

func bootCmd() tea.Cmd {
	return func() tea.Msg {
		client, creds, err := auth.AuthedClient()
		return bootMsg{client: client, creds: creds, err: err}
	}
}

func loadChatsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		boot, err := client.Bootstrap(ctx)
		if err != nil {
			return chatsMsg{err: err}
		}
		return chatsMsg{chats: boot.Chats}
	}
}

func openRoomCmd(client *api.Client, chatID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		title := chatID
		var messages []api.ChatMessage
		boot, err := client.ChatBootstrap(ctx, chatID)
		if err == nil {
			title = firstNonEmpty(boot.Chat.Title, boot.Chat.Name, chatID)
			messages = boot.Messages
		}
		members := loadMembers(ctx, client, chatID, boot)
		if err != nil && len(messages) == 0 {
			return roomMsg{err: err}
		}
		_ = config.SaveActiveChat(&config.ActiveChat{ChatID: chatID, Title: title})
		return roomMsg{title: title, messages: messages, members: members}
	}
}

func loadMembers(ctx context.Context, client *api.Client, chatID string, boot *api.ChatBootstrapResponse) []member {
	raw, err := client.ListMembers(ctx, chatID)
	if err != nil {
		out := []member{}
		if boot != nil {
			for _, name := range boot.Chat.MemberNames {
				out = append(out, member{DisplayName: name, MemberType: "ai", Slug: slugify(name)})
			}
		}
		return out
	}
	out := make([]member, 0, len(raw))
	for _, item := range raw {
		out = append(out, member{
			ID:          stringField(item, "id"),
			DisplayName: firstNonEmpty(stringField(item, "display_name"), stringField(item, "name")),
			Slug:        firstNonEmpty(stringField(item, "slug"), stringField(item, "username")),
			MemberType:  firstNonEmpty(stringField(item, "member_type"), stringField(item, "type")),
			Role:        stringField(item, "role"),
		})
	}
	return out
}

func pollCmd(client *api.Client, chatID string) tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		boot, err := client.ChatBootstrap(ctx, chatID)
		if err != nil {
			return pollMsg{err: err}
		}
		return pollMsg{messages: boot.Messages}
	})
}

func sendCmd(client *api.Client, chatID, content string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		msg, err := client.SendMessage(ctx, chatID, content)
		return sentMsg{msg: msg, err: err}
	}
}

func loginCmd(email, password string) tea.Cmd {
	return func() tea.Msg {
		err := auth.Login(config.BaseURL(), email, password)
		return loginDoneMsg{err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		if m.screen == screenRoom {
			m.refreshViewport()
		}
		return m, nil

	case bootMsg:
		if msg.err != nil {
			m.screen = screenLogin
			m.status = "Sign in to Salad"
			m.err = ""
			return m, nil
		}
		m.client = msg.client
		m.creds = msg.creds
		m.err = ""
		if m.chatID != "" {
			m.screen = screenRoom
			m.status = "Loading chat…"
			m.composer.Focus()
			return m, openRoomCmd(m.client, m.chatID)
		}
		m.screen = screenChats
		m.status = "Your Salad chats"
		m.chatLoad = true
		return m, loadChatsCmd(m.client)

	case loginDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Sign in failed"
			return m, nil
		}
		client, creds, err := auth.AuthedClient()
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.client = client
		m.creds = creds
		m.screen = screenChats
		m.status = "Your Salad chats"
		m.err = ""
		m.chatLoad = true
		return m, loadChatsCmd(m.client)

	case chatsMsg:
		m.chatLoad = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.chats = msg.chats
		m.err = ""
		if m.chatIdx >= len(m.chats) {
			m.chatIdx = 0
		}
		if len(m.chats) == 0 {
			m.status = "No chats yet — open one in Salad web, then refresh"
		} else {
			m.status = fmt.Sprintf("%d chats · think together", len(m.chats))
		}
		return m, nil

	case roomMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.screen = screenChats
			return m, loadChatsCmd(m.client)
		}
		m.chatTitle = msg.title
		m.messages = msg.messages
		m.members = msg.members
		m.err = ""
		m.status = "Live · same thread as Salad web"
		m.composer.Focus()
		m.refreshViewport()
		m.viewport.GotoBottom()
		return m, pollCmd(m.client, m.chatID)

	case pollMsg:
		if m.screen != screenRoom {
			return m, nil
		}
		if msg.err == nil && len(msg.messages) > 0 {
			changed := len(msg.messages) != len(m.messages)
			if !changed && len(msg.messages) > 0 && len(m.messages) > 0 {
				changed = msg.messages[len(msg.messages)-1].ID != m.messages[len(m.messages)-1].ID ||
					msg.messages[len(msg.messages)-1].Body != m.messages[len(m.messages)-1].Body
			}
			if changed {
				atBottom := m.viewport.AtBottom()
				m.messages = msg.messages
				m.refreshViewport()
				if atBottom {
					m.viewport.GotoBottom()
				}
			}
		}
		return m, pollCmd(m.client, m.chatID)

	case sentMsg:
		m.sending = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		if msg.msg != nil {
			m.messages = append(m.messages, *msg.msg)
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
		m.status = "Sent · waiting for collaborators…"
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenLogin:
			return m.updateLogin(msg)
		case screenChats:
			return m.updateChats(msg)
		case screenRoom:
			return m.updateRoom(msg)
		}
	}

	var cmd tea.Cmd
	if m.screen == screenRoom {
		m.composer, cmd = m.composer.Update(msg)
	}
	return m, cmd
}

func (m model) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "down":
		m.loginFocus = (m.loginFocus + 1) % 2
	case "shift+tab", "up":
		m.loginFocus = (m.loginFocus + 1) % 2
	case "enter":
		if strings.TrimSpace(m.loginEmail) == "" || m.loginPass == "" {
			m.err = "Email and password required"
			return m, nil
		}
		m.status = "Signing in…"
		m.err = ""
		return m, loginCmd(strings.TrimSpace(m.loginEmail), m.loginPass)
	case "backspace":
		if m.loginFocus == 0 && len(m.loginEmail) > 0 {
			m.loginEmail = m.loginEmail[:len(m.loginEmail)-1]
		}
		if m.loginFocus == 1 && len(m.loginPass) > 0 {
			m.loginPass = m.loginPass[:len(m.loginPass)-1]
		}
	default:
		if len(msg.Runes) == 0 {
			return m, nil
		}
		ch := string(msg.Runes)
		if m.loginFocus == 0 {
			m.loginEmail += ch
		} else {
			m.loginPass += ch
		}
	}
	return m, nil
}

func (m model) updateChats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		m.chatLoad = true
		m.status = "Refreshing…"
		return m, loadChatsCmd(m.client)
	case "up", "k":
		if m.chatIdx > 0 {
			m.chatIdx--
		}
	case "down", "j":
		if m.chatIdx < len(m.chats)-1 {
			m.chatIdx++
		}
	case "enter":
		if len(m.chats) == 0 {
			return m, nil
		}
		selected := m.chats[m.chatIdx]
		m.chatID = selected.ID
		m.chatTitle = selected.Title
		m.screen = screenRoom
		m.status = "Opening…"
		m.composer.SetValue("")
		m.composer.Focus()
		return m, openRoomCmd(m.client, m.chatID)
	}
	return m, nil
}

func (m model) updateRoom(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenChats
		m.status = "Your Salad chats"
		m.composer.Blur()
		m.chatLoad = true
		return m, loadChatsCmd(m.client)
	case "ctrl+p":
		m.status = participantsLine(m.members)
		return m, nil
	case "enter":
		return m.sendComposer()
	}

	// Prefer chat send on Enter; alt+enter inserts a newline.
	if msg.Type == tea.KeyEnter && !msg.Alt {
		return m.sendComposer()
	}
	if msg.Alt && msg.Type == tea.KeyEnter {
		m.composer.SetValue(m.composer.Value() + "\n")
		return m, nil
	}

	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	if strings.Contains(m.composer.Value(), "@") {
		m.status = "Mention: " + mentionHints(m.members)
	} else if m.status == "" || strings.HasPrefix(m.status, "Mention:") {
		m.status = "Live · same thread as Salad web"
	}
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(cmd, vpCmd)
}

func (m model) sendComposer() (tea.Model, tea.Cmd) {
	if m.sending || m.client == nil || strings.TrimSpace(m.chatID) == "" {
		return m, nil
	}
	if m.chatTitle == "" {
		return m, nil // still opening room
	}
	content := strings.TrimSpace(m.composer.Value())
	if content == "" {
		return m, nil
	}
	m.sending = true
	m.err = ""
	m.status = "Sending…"
	m.composer.SetValue("")
	return m, sendCmd(m.client, m.chatID, content)
}

func (m *model) layout() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	header := 3
	footer := 2
	composerH := 5
	vpH := m.height - header - footer - composerH
	if vpH < 5 {
		vpH = 5
	}
	m.viewport.Width = m.width - 2
	m.viewport.Height = vpH
	m.composer.SetWidth(m.width - 4)
	m.composer.SetHeight(3)
}

func (m *model) refreshViewport() {
	m.viewport.SetContent(m.renderTranscript())
}

func (m model) renderTranscript() string {
	if len(m.messages) == 0 {
		return theme.MutedText().Render("No messages yet. Say hello — same thread as Salad web.")
	}
	width := m.viewport.Width
	if width < 40 {
		width = 40
	}
	var b strings.Builder
	for _, msg := range m.messages {
		author := firstNonEmpty(msg.AuthorName, msg.Role, "member")
		body := strings.TrimSpace(msg.Body)
		if body == "" {
			body = "…"
		}
		wrapped := wordwrap.String(body, width-6)
		if isAssistant(msg) {
			b.WriteString(theme.AIHeader(author).Render("● "+author) + "\n")
			b.WriteString(theme.AIBody().Width(width-4).Render(wrapped) + "\n\n")
			continue
		}
		bubble := theme.UserBubble().Width(min(width-8, 72)).Render(author + "\n" + wrapped)
		b.WriteString(bubble + "\n\n")
	}
	return b.String()
}

func (m model) View() string {
	switch m.screen {
	case screenBoot:
		return theme.Header().Width(m.width).Render("∬alad") + "\n\n" + theme.MutedText().Render(m.status)
	case screenLogin:
		return m.viewLogin()
	case screenChats:
		return m.viewChats()
	case screenRoom:
		return m.viewRoom()
	default:
		return ""
	}
}

func (m model) viewLogin() string {
	w := max(m.width, 60)
	title := theme.Header().Width(w).Render("∬alad  ·  Terminal")
	sub := theme.MutedText().Render("Same account. Same chats. Think together from your repo.")
	emailLabel := "email"
	passLabel := "password"
	if m.loginFocus == 0 {
		emailLabel = theme.Brand().Render("▸ email")
	} else {
		passLabel = theme.Brand().Render("▸ password")
	}
	emailVal := m.loginEmail
	if emailVal == "" {
		emailVal = theme.MutedText().Render("you@company.com")
	}
	passVal := strings.Repeat("•", len(m.loginPass))
	if passVal == "" {
		passVal = theme.MutedText().Render("••••••••")
	}
	form := fmt.Sprintf("%s\n%s\n\n%s\n%s\n", emailLabel, emailVal, passLabel, passVal)
	box := theme.Composer().Width(min(w-4, 56)).Render(form)
	errLine := ""
	if m.err != "" {
		errLine = "\n" + theme.Error().Render(m.err)
	}
	host := theme.MutedText().Render("API " + config.BaseURL())
	help := theme.Footer().Width(w).Render("enter sign in · tab fields · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, title, sub, "", box, errLine, "", host, help)
}

func (m model) viewChats() string {
	w := max(m.width, 60)
	who := ""
	if m.creds != nil {
		who = firstNonEmpty(m.creds.Name, m.creds.Email)
	}
	title := theme.Header().Width(w).Render(fmt.Sprintf("∬alad  ·  %s", firstNonEmpty(who, "you")))
	sub := theme.MutedText().Render(m.status)
	var list strings.Builder
	if m.chatLoad {
		list.WriteString(theme.MutedText().Render("Loading chats…"))
	} else if len(m.chats) == 0 {
		list.WriteString(theme.MutedText().Render("No chats yet."))
	} else {
		limit := min(len(m.chats), max(8, m.height-8))
		start := 0
		if m.chatIdx >= limit {
			start = m.chatIdx - limit + 1
		}
		end := min(len(m.chats), start+limit)
		for i := start; i < end; i++ {
			chat := m.chats[i]
			unread := " "
			if chat.UnreadCount > 0 {
				unread = theme.UnreadDot().Render("•")
			}
			members := strings.Join(chat.MemberNames, ", ")
			if members == "" {
				members = "Salad chat"
			}
			line := fmt.Sprintf("%s %s", unread, firstNonEmpty(chat.Title, "Untitled"))
			meta := theme.MutedText().Render("  " + members)
			row := line + "\n" + meta
			if i == m.chatIdx {
				list.WriteString(theme.Selected().Width(w - 2).Render(row) + "\n")
			} else {
				list.WriteString(theme.ListItem().Width(w - 2).Render(row) + "\n")
			}
		}
	}
	errLine := ""
	if m.err != "" {
		errLine = theme.Error().Render(m.err) + "\n"
	}
	help := theme.Footer().Width(w).Render("↑↓ navigate · enter open · r refresh · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, title, sub, "", errLine+list.String(), help)
}

func (m model) viewRoom() string {
	w := max(m.width, 60)
	trust := "workspace untrusted"
	if m.workspaceOK {
		trust = "workspace trusted"
	}
	header := theme.Header().Width(w).Render(fmt.Sprintf("∬alad  ·  %s", firstNonEmpty(m.chatTitle, "Chat")))
	people := theme.MutedText().Render(participantsLine(m.members))
	body := m.viewport.View()
	composer := theme.Composer().Width(w - 2).Render(m.composer.View())
	status := m.status
	if m.sending {
		status = "Sending…"
	}
	if m.err != "" {
		status = m.err
	}
	footer := theme.Footer().Width(w).Render(fmt.Sprintf("%s  ·  %s  ·  enter send · alt+enter newline · esc chats · ctrl+p people · ctrl+c quit", status, trust))
	return lipgloss.JoinVertical(lipgloss.Left, header, people, body, composer, footer)
}

func isAssistant(msg api.ChatMessage) bool {
	role := strings.ToLower(msg.Role)
	if role == "assistant" || role == "ai" || role == "app" {
		return true
	}
	name := strings.ToLower(msg.AuthorName)
	for _, needle := range []string{"gpt", "claude", "gemini", "grok", "mistral", "llama", "groq", "chatgpt"} {
		if strings.Contains(name, needle) {
			return true
		}
	}
	return false
}

func participantsLine(members []member) string {
	if len(members) == 0 {
		return "participants loading…"
	}
	names := make([]string, 0, len(members))
	for _, mem := range members {
		label := firstNonEmpty(mem.DisplayName, mem.Slug)
		if strings.EqualFold(mem.MemberType, "ai") || strings.EqualFold(mem.MemberType, "app") {
			label = "AI " + label
		}
		names = append(names, label)
	}
	return "With " + strings.Join(names, " · ")
}

func mentionHints(members []member) string {
	parts := make([]string, 0, len(members))
	for _, mem := range members {
		slug := firstNonEmpty(mem.Slug, slugify(mem.DisplayName))
		if slug != "" {
			parts = append(parts, "@"+slug)
		}
	}
	if len(parts) == 0 {
		return "@someone"
	}
	if len(parts) > 6 {
		parts = parts[:6]
	}
	return strings.Join(parts, " ")
}

func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprint(v)
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
