package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/salad-ai/salad-terminal/internal/api"
	"github.com/salad-ai/salad-terminal/internal/auth"
	"github.com/salad-ai/salad-terminal/internal/bridge"
	"github.com/salad-ai/salad-terminal/internal/config"
	"github.com/salad-ai/salad-terminal/internal/realtime"
	"github.com/salad-ai/salad-terminal/internal/theme"
	"github.com/salad-ai/salad-terminal/internal/workspace"
)

// Options controls how Salad Terminal starts a session.
// Industry default (Claude Code): bare launch = new session;
// --continue = last for this folder; --resume = picker.
type Options struct {
	ChatID        string
	ForceResume   bool // salad --resume → picker
	ForceContinue bool // salad --continue → last chat for this folder
	ForceNew      bool // salad / salad new → AI picker → create
}

type screen int

const (
	screenBoot screen = iota
	screenLogin
	screenChats
	screenNewAI
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

	loginEmail string
	loginPass  string
	loginFocus int

	chats        []api.ChatPreview
	chatIdx      int // picker: 0 = New chat, 1.. = chats[i-1]
	chatLoad     bool
	chatCreating bool
	forceResume   bool
	forceContinue bool
	forceNew      bool
	pickerInited  bool

	aiProducts []api.AIProduct
	aiSelected map[string]bool
	aiIdx      int
	aiLoad     bool
	aiShowMore bool   // false = family defaults only; true = full chat catalog
	aiPurpose  string // "create" | "add"

	chatID       string
	chatTitle    string
	members      []member
	messages     []api.ChatMessage
	viewport     viewport.Model
	composer     textarea.Model
	sending      bool
	workspaceOK  bool
	workspaceDir string
	live         string // "ws" | "poll" | ""
	wsClient     *realtime.Client
	wsEvents     <-chan realtime.Event
	focusFiles   []string
	attachTools  bool

	mentionOpen bool
	mentionIdx  int
	mentionQ    string
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
	wsEventMsg struct {
		evt realtime.Event
	}
	wsReadyMsg struct {
		ok  bool
		err error
	}
	sentMsg struct {
		msg *api.ChatMessage
		err error
	}
	loginDoneMsg struct {
		err error
	}
	createdChatMsg struct {
		chat *api.ChatPreview
		err  error
	}
	aiProductsMsg struct {
		products []api.AIProduct
		err      error
	}
	addedAIsMsg struct {
		count int
		err   error
	}
)

func Run(initialChatID string) error {
	return RunOptions(Options{ChatID: initialChatID})
}

func RunOptions(opts Options) error {
	m := newModel(opts)
	programOpts := []tea.ProgramOption{}
	// SALAD_SIMPLE=1 keeps output in the normal screen buffer so Cursor/agent
	// terminals can display the session (alt-screen needs a real interactive TTY).
	if os.Getenv("SALAD_SIMPLE") == "" {
		programOpts = append(programOpts, tea.WithAltScreen(), tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, programOpts...)
	finalModel, err := p.Run()
	if fm, ok := finalModel.(model); ok && fm.wsClient != nil {
		fm.wsClient.Close()
	} else if m.wsClient != nil {
		m.wsClient.Close()
	}
	return err
}

func newModel(opts Options) model {
	ta := textarea.New()
	ta.Placeholder = "Message…  @mention · /add for more AIs"
	ta.ShowLineNumbers = false
	ta.Prompt = "∬ "
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	root, _ := workspace.ResolveRoot("")
	return model{
		screen:       screenBoot,
		status:       "Opening Salad…",
		composer:     ta,
		viewport:     viewport.New(80, 20),
		chatID:        strings.TrimSpace(opts.ChatID),
		forceResume:   opts.ForceResume,
		forceContinue: opts.ForceContinue,
		forceNew:      opts.ForceNew,
		workspaceDir:  root,
		workspaceOK:  workspace.IsTrusted(root),
		// Opt-in: /git /read /diff or ctrl+t. Avoid shipping git dumps on every send.
		attachTools: false,
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
		// Prefer full messages API (up to 100) over bootstrap's last-50 window.
		if listed, listErr := client.ListMessages(ctx, chatID, ""); listErr == nil && len(listed) > 0 {
			messages = listed
		}
		members := loadMembers(ctx, client, chatID, boot)
		if err != nil && len(messages) == 0 {
			return roomMsg{err: err}
		}
		_ = config.SaveActiveChat(&config.ActiveChat{ChatID: chatID, Title: title})
		return roomMsg{title: title, messages: messages, members: members}
	}
}

func fetchRoomMessages(ctx context.Context, client *api.Client, chatID string) ([]api.ChatMessage, error) {
	if listed, err := client.ListMessages(ctx, chatID, ""); err == nil && len(listed) > 0 {
		return listed, nil
	}
	boot, err := client.ChatBootstrap(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return boot.Messages, nil
}

func createChatCmd(client *api.Client, workspaceDir string, aiSlugs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		chat, err := client.CreateChat(ctx, defaultTerminalChatName(workspaceDir), aiSlugs)
		return createdChatMsg{chat: chat, err: err}
	}
}

func addAIsCmd(client *api.Client, chatID string, aiSlugs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		added := 0
		var lastErr error
		for _, slug := range aiSlugs {
			if err := client.AddAIMember(ctx, chatID, slug); err != nil {
				lastErr = err
				continue
			}
			added++
		}
		if added == 0 && lastErr != nil {
			return addedAIsMsg{err: lastErr}
		}
		return addedAIsMsg{count: added, err: lastErr}
	}
}

func loadAIProductsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		products, err := client.ListAIProducts(ctx)
		return aiProductsMsg{products: products, err: err}
	}
}

// Web new-chat family defaults (chat category).
var defaultAIProductSlugs = []string{
	"claude-sonnet",
	"gpt-5-4",
	"gemini-pro",
	"grok-4",
	"mistral-medium",
	"groq-compound-mini",
	"llama-3-3-70b",
}

func (m *model) beginNewChat() tea.Cmd {
	m.screen = screenNewAI
	m.aiPurpose = "create"
	m.aiLoad = true
	m.aiIdx = 0
	m.aiShowMore = false
	m.aiSelected = map[string]bool{}
	m.chatCreating = false
	m.status = ""
	m.err = ""
	return loadAIProductsCmd(m.client)
}

func (m *model) beginAddAI() tea.Cmd {
	m.screen = screenNewAI
	m.aiPurpose = "add"
	m.aiLoad = true
	m.aiIdx = 0
	m.aiShowMore = false
	m.aiSelected = map[string]bool{}
	m.chatCreating = false
	m.status = ""
	m.err = ""
	return loadAIProductsCmd(m.client)
}

func (m *model) existingAISlugSet() map[string]bool {
	out := map[string]bool{}
	for _, mem := range m.members {
		if !strings.EqualFold(mem.MemberType, "ai") && !strings.EqualFold(mem.MemberType, "app") {
			continue
		}
		if s := strings.ToLower(strings.TrimSpace(mem.Slug)); s != "" {
			out[s] = true
		}
		// Display names like "GPT-5.4" → try common slug forms
		if s := slugify(mem.DisplayName); s != "" {
			out[s] = true
		}
	}
	return out
}

func (m *model) visibleAIProducts() []api.AIProduct {
	bySlug := map[string]api.AIProduct{}
	for _, p := range m.aiProducts {
		bySlug[p.Slug] = p
	}
	existing := map[string]bool{}
	if m.aiPurpose == "add" {
		existing = m.existingAISlugSet()
	}

	var out []api.AIProduct
	if !m.aiShowMore {
		// Family defaults only (same as web new-chat default versions).
		for _, slug := range defaultAIProductSlugs {
			p, ok := bySlug[slug]
			if !ok {
				continue
			}
			if existing[strings.ToLower(slug)] {
				continue
			}
			out = append(out, p)
		}
	} else {
		for _, p := range m.aiProducts {
			if !strings.EqualFold(p.Category, "chat") {
				continue
			}
			if existing[strings.ToLower(p.Slug)] {
				continue
			}
			out = append(out, p)
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Provider != out[j].Provider {
				return out[i].Provider < out[j].Provider
			}
			return out[i].DisplayName < out[j].DisplayName
		})
	}
	return out
}

func (m *model) selectedAISlugs() []string {
	visible := m.visibleAIProducts()
	out := make([]string, 0, len(visible))
	for _, p := range visible {
		if m.aiSelected[p.Slug] && p.HasAccess {
			out = append(out, p.Slug)
		}
	}
	return out
}

func applyDefaultAISelection(products []api.AIProduct) map[string]bool {
	selected := map[string]bool{}
	avail := map[string]api.AIProduct{}
	for _, p := range products {
		if p.HasAccess && strings.EqualFold(p.Category, "chat") {
			avail[p.Slug] = p
		}
	}
	// Start with one AI (like Claude Code / Codex). Space selects more; a selects all.
	for _, slug := range defaultAIProductSlugs {
		if _, ok := avail[slug]; ok {
			selected[slug] = true
			return selected
		}
	}
	for _, p := range products {
		if p.HasAccess && strings.EqualFold(p.Category, "chat") {
			selected[p.Slug] = true
			break
		}
	}
	return selected
}

func defaultTerminalChatName(workspaceDir string) string {
	base := filepath.Base(filepath.Clean(workspaceDir))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "New chat"
	}
	if len(base) > 100 {
		base = base[:100]
	}
	return base
}

func displayChatTitle(title string) string {
	t := strings.TrimSpace(title)
	t = strings.TrimSuffix(t, " · Terminal")
	t = strings.TrimSpace(t)
	return firstNonEmpty(t, "Chat")
}

func resolveContinueChat(workspaceDir string) (chatID, title string) {
	if id, t, err := config.WorkspaceChatID(workspaceDir); err == nil && id != "" {
		return id, t
	}
	if active, err := config.LoadActiveChat(); err == nil && active.ChatID != "" {
		return active.ChatID, active.Title
	}
	return "", ""
}

func (m *model) openSelectedChat(id, title string) tea.Cmd {
	m.chatID = id
	m.chatTitle = title
	m.screen = screenRoom
	m.status = "Opening…"
	m.err = ""
	m.composer.SetValue("")
	m.composer.Focus()
	_ = config.BindWorkspace(m.workspaceDir, id, title)
	return openRoomCmd(m.client, id)
}

func (m *model) showResumePicker(status string) tea.Cmd {
	m.screen = screenChats
	m.status = status
	m.chatLoad = true
	m.pickerInited = false
	m.chatIdx = 0
	return loadChatsCmd(m.client)
}

func (m *model) afterAuth() tea.Cmd {
	wsCmd := wsListenCmd(config.BaseURL(), m.creds.AccessToken)

	// Explicit resume picker (claude --resume).
	if m.forceResume {
		return tea.Batch(m.showResumePicker("↑↓ open · n new"), wsCmd)
	}

	// Jump to a specific chat id.
	if m.chatID != "" {
		m.screen = screenRoom
		m.status = "Opening…"
		_ = config.BindWorkspace(m.workspaceDir, m.chatID, m.chatTitle)
		return tea.Batch(openRoomCmd(m.client, m.chatID), wsCmd)
	}

	// Continue last chat for this folder (claude --continue).
	if m.forceContinue {
		if id, title := resolveContinueChat(m.workspaceDir); id != "" {
			m.chatID = id
			m.chatTitle = title
			m.screen = screenRoom
			m.status = "Continuing…"
			_ = config.BindWorkspace(m.workspaceDir, m.chatID, m.chatTitle)
			return tea.Batch(openRoomCmd(m.client, m.chatID), wsCmd)
		}
		return tea.Batch(m.showResumePicker("No recent chat here · n new · enter open"), wsCmd)
	}

	// Default bare `salad` / `salad new`: new session (claude with no flags).
	return tea.Batch(m.beginNewChat(), wsCmd)
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
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages, err := fetchRoomMessages(ctx, client, chatID)
		if err != nil {
			return pollMsg{err: err}
		}
		return pollMsg{messages: messages}
	})
}

func wsListenCmd(baseURL, token string) tea.Cmd {
	return func() tea.Msg {
		client := realtime.New(baseURL, token)
		ch, err := client.Connect()
		if err != nil {
			return wsReadyMsg{ok: false, err: err}
		}
		return wsSessionMsg{client: client, ch: ch}
	}
}

type wsSessionMsg struct {
	client *realtime.Client
	ch     <-chan realtime.Event
}

func nextWSCmd(ch <-chan realtime.Event) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return wsReadyMsg{ok: false, err: fmt.Errorf("websocket closed")}
		}
		return wsEventMsg{evt: evt}
	}
}

func sendCmd(client *api.Client, chatID string, req api.SendMessageRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		msg, err := client.SendMessageRequest(ctx, chatID, req)
		return sentMsg{msg: msg, err: err}
	}
}

func loginCmd(email, password string) tea.Cmd {
	return func() tea.Msg {
		return loginDoneMsg{err: auth.Login(config.BaseURL(), email, password)}
	}
}

func loginGoogleCmd() tea.Cmd {
	return func() tea.Msg {
		return loginDoneMsg{err: auth.LoginGoogleBrowser(config.BaseURL())}
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
			return m, nil
		}
		m.client = msg.client
		m.creds = msg.creds
		return m, m.afterAuth()

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
		m.err = ""
		return m, m.afterAuth()

	case chatsMsg:
		m.chatLoad = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.chats = msg.chats
		m.err = ""
		maxIdx := len(m.chats) // 0 = New, 1..len = chats
		if !m.pickerInited {
			m.pickerInited = true
			if len(m.chats) > 0 {
				m.chatIdx = 1 // highlight most recent resume target
			} else {
				m.chatIdx = 0
			}
		}
		if m.chatIdx > maxIdx {
			m.chatIdx = maxIdx
		}
		if len(m.chats) == 0 {
			m.status = "No chats yet — n to start one"
		} else {
			m.status = ""
		}
		return m, nil

	case aiProductsMsg:
		m.aiLoad = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Could not load AIs"
			return m, nil
		}
		m.aiProducts = msg.products
		if m.aiPurpose == "add" {
			m.aiSelected = map[string]bool{}
		} else {
			m.aiSelected = applyDefaultAISelection(msg.products)
		}
		m.aiIdx = 0
		visible := m.visibleAIProducts()
		if m.aiPurpose == "add" {
			m.status = fmt.Sprintf("%d available", len(visible))
		} else {
			m.status = fmt.Sprintf("%d selected", len(m.selectedAISlugs()))
		}
		m.err = ""
		return m, nil

	case addedAIsMsg:
		m.chatCreating = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Could not add AIs"
			return m, nil
		}
		m.screen = screenRoom
		m.status = fmt.Sprintf("Added %d AI(s) — @mention them", msg.count)
		m.err = ""
		return m, openRoomCmd(m.client, m.chatID)

	case createdChatMsg:
		m.chatCreating = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Could not create chat"
			m.screen = screenNewAI
			return m, nil
		}
		title := firstNonEmpty(msg.chat.Title, "Terminal")
		return m, m.openSelectedChat(msg.chat.ID, title)

	case roomMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.forceResume = true
			return m, m.showResumePicker("Could not open chat — pick another")
		}
		m.chatTitle = msg.title
		m.messages = msg.messages
		m.members = msg.members
		m.err = ""
		m.status = ""
		_ = config.BindWorkspace(m.workspaceDir, m.chatID, m.chatTitle)
		m.composer.Focus()
		m.refreshViewport()
		m.viewport.GotoBottom()
		cmds := []tea.Cmd{pollCmd(m.client, m.chatID)}
		return m, tea.Batch(cmds...)

	case wsSessionMsg:
		if m.wsClient != nil {
			m.wsClient.Close()
		}
		m.wsClient = msg.client
		m.wsEvents = msg.ch
		m.live = "ws"
		return m, nextWSCmd(m.wsEvents)

	case wsReadyMsg:
		if !msg.ok {
			m.live = "poll"
			m.wsEvents = nil
			return m, nil
		}
		return m, nil

	case wsEventMsg:
		if m.wsEvents == nil {
			return m, nil
		}
		cmd := nextWSCmd(m.wsEvents)
		if m.screen == screenRoom && (msg.evt.ChatID == "" || msg.evt.ChatID == m.chatID) && realtime.IsChatSignal(msg.evt) {
			return m, tea.Batch(cmd, refreshRoomCmd(m.client, m.chatID))
		}
		return m, cmd

	case pollMsg:
		if m.screen != screenRoom {
			return m, nil
		}
		if msg.err == nil {
			m.applyMessages(msg.messages)
		}
		if m.live != "ws" {
			m.live = "poll"
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
			m.applyMessages([]api.ChatMessage{*msg.msg})
			m.viewport.GotoBottom()
		}
		m.status = "Sent · waiting for replies…"
		return m, refreshRoomCmd(m.client, m.chatID)

	case tea.MouseMsg:
		if m.screen != screenRoom {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.LineUp(3)
		case tea.MouseButtonWheelDown:
			m.viewport.LineDown(3)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenLogin:
			return m.updateLogin(msg)
		case screenChats:
			return m.updateChats(msg)
		case screenNewAI:
			return m.updateNewAI(msg)
		case screenRoom:
			return m.updateRoom(msg)
		}
	}
	return m, nil
}

func refreshRoomCmd(client *api.Client, chatID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages, err := fetchRoomMessages(ctx, client, chatID)
		if err != nil {
			return pollMsg{err: err}
		}
		return pollMsg{messages: messages}
	}
}

func loadOlderMessagesCmd(client *api.Client, chatID, beforeID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		messages, err := client.ListMessages(ctx, chatID, beforeID)
		if err != nil {
			return pollMsg{err: err}
		}
		return pollMsg{messages: messages}
	}
}

func messageKey(msg api.ChatMessage) string {
	if id := strings.TrimSpace(msg.ID); id != "" {
		return id
	}
	return fmt.Sprintf("local:%s:%s:%d", msg.AuthorName, msg.Body, msg.CreatedAt.UnixNano())
}

func preferMessage(a, b api.ChatMessage) api.ChatMessage {
	// Prefer the fuller / more complete version of the same message.
	out := b
	if len(strings.TrimSpace(a.Body)) > len(strings.TrimSpace(b.Body)) {
		out = a
		out.Status = firstNonEmpty(b.Status, a.Status)
	}
	if out.AuthorName == "" {
		out.AuthorName = a.AuthorName
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = a.CreatedAt
	}
	return out
}

func mergeMessages(existing, incoming []api.ChatMessage) []api.ChatMessage {
	byID := make(map[string]api.ChatMessage, len(existing)+len(incoming))
	for _, msg := range existing {
		byID[messageKey(msg)] = msg
	}
	for _, msg := range incoming {
		key := messageKey(msg)
		if prev, ok := byID[key]; ok {
			byID[key] = preferMessage(prev, msg)
		} else {
			byID[key] = msg
		}
	}
	out := make([]api.ChatMessage, 0, len(byID))
	for _, msg := range byID {
		out = append(out, msg)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func messagesChanged(a, b []api.ChatMessage) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Body != b[i].Body || a[i].Status != b[i].Status {
			return true
		}
	}
	return false
}

func (m *model) applyMessages(incoming []api.ChatMessage) {
	if len(incoming) == 0 {
		return
	}
	merged := mergeMessages(m.messages, incoming)
	if !messagesChanged(m.messages, merged) {
		return
	}
	atBottom := m.viewport.AtBottom()
	yOffset := m.viewport.YOffset
	m.messages = merged
	m.refreshViewport()
	if atBottom {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(yOffset)
	}
}

func (m model) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "g":
		if m.loginFocus == 0 && m.loginEmail == "" {
			m.status = "Opening Google…"
			m.err = ""
			return m, loginGoogleCmd()
		}
	case "tab", "down":
		m.loginFocus = (m.loginFocus + 1) % 2
	case "shift+tab", "up":
		m.loginFocus = (m.loginFocus + 1) % 2
	case "enter":
		if strings.TrimSpace(m.loginEmail) == "" || m.loginPass == "" {
			m.err = "Email and password required — or press g for Google"
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
	if m.chatCreating {
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		return m, nil
	}
	maxIdx := len(m.chats) // 0 = New
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		m.chatLoad = true
		m.pickerInited = false
		return m, loadChatsCmd(m.client)
	case "n":
		return m, m.beginNewChat()
	case "up", "k":
		if m.chatIdx > 0 {
			m.chatIdx--
		}
	case "down", "j":
		if m.chatIdx < maxIdx {
			m.chatIdx++
		}
	case "enter":
		if m.chatIdx == 0 {
			return m, m.beginNewChat()
		}
		if m.chatIdx-1 >= len(m.chats) {
			return m, nil
		}
		selected := m.chats[m.chatIdx-1]
		return m, m.openSelectedChat(selected.ID, firstNonEmpty(selected.Title, "Untitled"))
	default:
		// 1-9 jump into existing chats and open (numbered menu pattern)
		if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= 9 && n <= len(m.chats) {
			selected := m.chats[n-1]
			m.chatIdx = n
			return m, m.openSelectedChat(selected.ID, firstNonEmpty(selected.Title, "Untitled"))
		}
	}
	return m, nil
}

func (m model) updateNewAI(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.chatCreating {
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		return m, nil
	}
	visible := m.visibleAIProducts()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.aiPurpose == "add" {
			m.screen = screenRoom
			m.status = ""
			m.err = ""
			m.composer.Focus()
			return m, nil
		}
		m.forceResume = true
		return m, m.showResumePicker("↑↓ open a chat · n new")
	case "up", "k":
		if m.aiIdx > 0 {
			m.aiIdx--
		}
	case "down", "j":
		if m.aiIdx < len(visible)-1 {
			m.aiIdx++
		}
	case " ", "space":
		if len(visible) == 0 || m.aiIdx < 0 || m.aiIdx >= len(visible) {
			return m, nil
		}
		p := visible[m.aiIdx]
		if !p.HasAccess {
			m.err = p.DisplayName + " needs a higher plan"
			return m, nil
		}
		if m.aiSelected == nil {
			m.aiSelected = map[string]bool{}
		}
		m.aiSelected[p.Slug] = !m.aiSelected[p.Slug]
		m.err = ""
		m.status = fmt.Sprintf("%d selected", len(m.selectedAISlugs()))
	case "a":
		// Select all currently visible AIs.
		m.aiSelected = map[string]bool{}
		for _, p := range visible {
			if p.HasAccess {
				m.aiSelected[p.Slug] = true
			}
		}
		m.status = fmt.Sprintf("%d selected", len(m.selectedAISlugs()))
		m.err = ""
	case "m":
		m.aiShowMore = !m.aiShowMore
		m.aiIdx = 0
		if m.aiShowMore {
			m.status = "All chat models · m for defaults only"
		} else {
			m.status = "Family defaults · m for more models"
		}
	case "enter":
		slugs := m.selectedAISlugs()
		if len(slugs) == 0 {
			m.err = "Pick at least one AI (space)"
			return m, nil
		}
		m.chatCreating = true
		m.err = ""
		if m.aiPurpose == "add" {
			m.status = "Adding…"
			return m, addAIsCmd(m.client, m.chatID, slugs)
		}
		m.status = "Creating…"
		return m, createChatCmd(m.client, m.workspaceDir, slugs)
	case "r":
		m.aiLoad = true
		return m, loadAIProductsCmd(m.client)
	}
	return m, nil
}

func (m model) updateRoom(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mentionOpen {
		return m.updateMention(msg)
	}

	composerEmpty := strings.TrimSpace(m.composer.Value()) == ""

	switch msg.String() {
	case "ctrl+c":
		if m.wsClient != nil {
			m.wsClient.Close()
		}
		return m, tea.Quit
	case "q":
		// Quit only when composer is empty — otherwise type "q".
		if composerEmpty {
			if m.wsClient != nil {
				m.wsClient.Close()
			}
			return m, tea.Quit
		}
	case "esc":
		m.composer.Blur()
		m.forceResume = true
		return m, m.showResumePicker("Resume another Salad chat · n new · enter open")
	case "ctrl+p":
		m.status = participantsLine(m.members)
		return m, nil
	case "ctrl+t":
		m.attachTools = !m.attachTools
		if m.attachTools {
			m.status = "Local tools on for next send"
		} else {
			m.status = "Local tools off for next send"
		}
		return m, nil
	case "pgup", "ctrl+u":
		wasTop := m.viewport.AtTop()
		m.viewport.HalfViewUp()
		if wasTop || m.viewport.AtTop() {
			return m, m.requestOlderMessages()
		}
		return m, nil
	case "pgdown", "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil
	case "up":
		// Empty composer: ↑ scrolls history (don't trap keys in the textarea).
		if composerEmpty {
			wasTop := m.viewport.AtTop()
			m.viewport.LineUp(1)
			if wasTop || m.viewport.AtTop() {
				return m, m.requestOlderMessages()
			}
			return m, nil
		}
	case "down":
		if composerEmpty {
			m.viewport.LineDown(1)
			return m, nil
		}
	case "enter":
		return m.sendComposer()
	}

	if msg.Type == tea.KeyEnter && !msg.Alt {
		return m.sendComposer()
	}
	if msg.Alt && msg.Type == tea.KeyEnter {
		m.composer.SetValue(m.composer.Value() + "\n")
		return m, nil
	}

	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	value := m.composer.Value()

	// Slash local tools (do not send to chat).
	if strings.HasPrefix(strings.TrimSpace(value), "/") && strings.HasSuffix(value, "\n") {
		return m.runSlash(strings.TrimSpace(value))
	}

	if at := strings.LastIndex(value, "@"); at >= 0 {
		tail := value[at+1:]
		if !strings.ContainsAny(tail, " \n\t") {
			m.mentionOpen = true
			m.mentionQ = strings.ToLower(tail)
			m.mentionIdx = 0
			m.status = "Mention someone"
			return m, cmd
		}
	}
	m.mentionOpen = false
	return m, cmd
}

func (m *model) requestOlderMessages() tea.Cmd {
	if m.client == nil || m.chatID == "" || len(m.messages) == 0 {
		return nil
	}
	oldestID := strings.TrimSpace(m.messages[0].ID)
	if oldestID == "" {
		return nil
	}
	m.status = "Loading earlier messages…"
	return loadOlderMessagesCmd(m.client, m.chatID, oldestID)
}

func (m model) updateMention(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredMentions()
	switch msg.String() {
	case "esc":
		m.mentionOpen = false
		return m, nil
	case "up":
		if m.mentionIdx > 0 {
			m.mentionIdx--
		}
		return m, nil
	case "down", "tab":
		if m.mentionIdx < len(filtered)-1 {
			m.mentionIdx++
		}
		return m, nil
	case "enter":
		if len(filtered) == 0 {
			m.mentionOpen = false
			return m, nil
		}
		pick := filtered[m.mentionIdx]
		m.insertMention(pick)
		m.mentionOpen = false
		return m, nil
	case "backspace":
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		value := m.composer.Value()
		if at := strings.LastIndex(value, "@"); at < 0 {
			m.mentionOpen = false
		} else {
			m.mentionQ = strings.ToLower(value[at+1:])
		}
		return m, cmd
	default:
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		value := m.composer.Value()
		if at := strings.LastIndex(value, "@"); at >= 0 {
			tail := value[at+1:]
			if strings.ContainsAny(tail, " \n\t") {
				m.mentionOpen = false
			} else {
				m.mentionQ = strings.ToLower(tail)
			}
		} else {
			m.mentionOpen = false
		}
		return m, cmd
	}
}

func (m *model) insertMention(mem member) {
	value := m.composer.Value()
	at := strings.LastIndex(value, "@")
	if at < 0 {
		return
	}
	slug := firstNonEmpty(mem.Slug, slugify(mem.DisplayName))
	m.composer.SetValue(value[:at] + "@" + slug + " ")
	m.composer.CursorEnd()
}

func (m model) filteredMentions() []member {
	q := m.mentionQ
	out := make([]member, 0, len(m.members))
	for _, mem := range m.members {
		hay := strings.ToLower(mem.DisplayName + " " + mem.Slug)
		if q == "" || strings.Contains(hay, q) {
			out = append(out, mem)
		}
	}
	return out
}

func (m model) runSlash(line string) (tea.Model, tea.Cmd) {
	m.composer.SetValue("")
	parts := strings.Fields(strings.TrimPrefix(line, "/"))
	if len(parts) == 0 {
		return m, nil
	}
	switch parts[0] {
	case "git", "status":
		out, err := workspace.GitStatus(m.workspaceDir)
		if err != nil {
			m.err = err.Error()
		} else {
			m.status = "git status attached for next send"
			m.focusFiles = nil
			_ = out
			// stash via diagnostics by marking tools on
			m.attachTools = true
			m.err = ""
			// show in transcript as local note
			m.messages = append(m.messages, api.ChatMessage{AuthorName: "local", Role: "system", Body: "git status:\n" + out})
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
	case "diff":
		out, err := workspace.GitDiff(m.workspaceDir)
		if err != nil {
			m.err = err.Error()
		} else {
			m.attachTools = true
			m.messages = append(m.messages, api.ChatMessage{AuthorName: "local", Role: "system", Body: "git diff --stat:\n" + out})
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
	case "read":
		if len(parts) < 2 {
			m.err = "usage: /read <path>"
			return m, nil
		}
		rel := parts[1]
		content, err := workspace.ReadFile(m.workspaceDir, rel)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.focusFiles = appendUnique(m.focusFiles, filepath.Clean(rel))
		m.attachTools = true
		m.messages = append(m.messages, api.ChatMessage{AuthorName: "local", Role: "system", Body: "attached " + rel + "\n" + truncate(content, 1200)})
		m.refreshViewport()
		m.viewport.GotoBottom()
		m.status = "Attached " + rel + " for next send"
	case "trust":
		if err := workspace.Trust(m.workspaceDir); err != nil {
			m.err = err.Error()
		} else {
			m.workspaceOK = true
			m.status = "Workspace trusted"
		}
	case "tools":
		m.attachTools = !m.attachTools
		m.status = fmt.Sprintf("attachTools=%v", m.attachTools)
	case "new":
		return m, m.beginNewChat()
	case "add":
		return m, m.beginAddAI()
	case "resume", "chats":
		m.composer.Blur()
		m.forceResume = true
		return m, m.showResumePicker("↑↓ open a chat · n new")
	default:
		m.err = "try /add · /new · /resume · /git · /read · /trust"
	}
	return m, nil
}

func (m model) sendComposer() (tea.Model, tea.Cmd) {
	if m.sending || m.client == nil || m.chatID == "" {
		return m, nil
	}
	content := strings.TrimSpace(m.composer.Value())
	if content == "" {
		return m, nil
	}
	// Intercept slash commands typed without trailing newline handling.
	if strings.HasPrefix(content, "/") {
		return m.runSlash(content)
	}

	req := api.SendMessageRequest{
		Content: content,
		Metadata: map[string]any{
			"client_surface": "salad_terminal",
			"terminal": map[string]any{
				"workspace_trusted": m.workspaceOK,
				"attach_tools":      m.attachTools,
			},
		},
	}
	mentions, tagged, hint := parseMentions(content, m.members)
	req.ExplicitMentions = mentions
	req.TaggedMembers = tagged
	req.TargetHint = hint

	if m.attachTools && m.workspaceOK {
		if codeCtx, _, err := bridge.BuildCodeContext(m.workspaceDir, m.focusFiles); err == nil {
			req.CodeContext = codeCtx
		}
	}

	m.sending = true
	m.err = ""
	m.status = "Sending…"
	m.composer.SetValue("")
	m.mentionOpen = false
	return m, sendCmd(m.client, m.chatID, req)
}

func parseMentions(content string, members []member) ([]api.ExplicitMention, []string, *api.TargetHint) {
	var mentions []api.ExplicitMention
	var tagged []string
	var slugs []string
	var names []string
	var ids []string
	lowerContent := strings.ToLower(content)
	seen := map[string]bool{}
	for _, mem := range members {
		slug := firstNonEmpty(mem.Slug, slugify(mem.DisplayName))
		matchedToken := ""
		matchedIdx := -1
		for _, alias := range mentionAliases(mem) {
			token := "@" + alias
			idx := strings.Index(lowerContent, strings.ToLower(token))
			if idx < 0 {
				continue
			}
			// Prefer earlier index; at the same index prefer the longest alias.
			prevAliasLen := len(strings.TrimPrefix(matchedToken, "@"))
			if matchedIdx < 0 || idx < matchedIdx || (idx == matchedIdx && len(alias) > prevAliasLen) {
				matchedToken = token
				matchedIdx = idx
			}
		}
		if matchedIdx < 0 || seen[mem.ID] {
			continue
		}
		seen[mem.ID] = true
		mentions = append(mentions, api.ExplicitMention{
			MemberID: mem.ID,
			Token:    matchedToken,
			Start:    matchedIdx,
			End:      matchedIdx + len(matchedToken),
		})
		if mem.ID != "" {
			tagged = append(tagged, mem.ID)
			ids = append(ids, mem.ID)
		}
		slugs = append(slugs, slug)
		names = append(names, mem.DisplayName)
	}
	if strings.Contains(lowerContent, "@everyone") {
		tagged = append(tagged, "@everyone")
	}
	if len(mentions) == 0 && len(tagged) == 0 {
		return nil, nil, nil
	}
	hint := &api.TargetHint{
		MemberIDs:    ids,
		Slugs:        slugs,
		DisplayNames: names,
		Source:       "salad_terminal",
	}
	return mentions, tagged, hint
}

func mentionAliases(mem member) []string {
	slug := strings.TrimSpace(firstNonEmpty(mem.Slug, slugify(mem.DisplayName)))
	if slug == "" {
		return nil
	}
	out := []string{slug}
	lower := strings.ToLower(slug + " " + mem.DisplayName)
	add := func(alias string) {
		alias = strings.TrimSpace(strings.ToLower(alias))
		if alias == "" || alias == strings.ToLower(slug) {
			return
		}
		for _, existing := range out {
			if strings.EqualFold(existing, alias) {
				return
			}
		}
		out = append(out, alias)
	}
	switch {
	case strings.Contains(lower, "gpt") || strings.Contains(lower, "chatgpt"):
		add("gpt")
		add("chatgpt")
	case strings.Contains(lower, "claude"):
		add("claude")
	case strings.Contains(lower, "gemini"):
		add("gemini")
	case strings.Contains(lower, "grok"):
		add("grok")
	case strings.Contains(lower, "mistral"):
		add("mistral")
	}
	return out
}

func (m *model) layout() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	composerH := 5
	mentionH := 0
	if m.mentionOpen {
		mentionH = 6
	}
	vpH := m.height - 3 - 2 - composerH - mentionH
	if vpH < 5 {
		vpH = 5
	}
	m.viewport.Width = m.width - 2
	m.viewport.Height = vpH
	m.composer.SetWidth(m.width - 4)
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
			switch strings.ToLower(msg.Status) {
			case "processing", "streaming", "queued", "pending":
				body = "thinking…"
			default:
				if isAssistant(msg) {
					body = "thinking…"
				} else {
					body = "…"
				}
			}
		}
		wrapped := wordwrap.String(body, width-6)
		if strings.EqualFold(msg.Role, "system") || author == "local" {
			b.WriteString(theme.MutedText().Render("⚙ "+wrapped) + "\n\n")
			continue
		}
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
		return theme.Banner(max(m.width, 40)) + "\n\n" + theme.MutedText().Render(m.status)
	case screenLogin:
		return m.viewLogin()
	case screenChats:
		return m.viewChats()
	case screenNewAI:
		return m.viewNewAI()
	case screenRoom:
		return m.viewRoom()
	default:
		return ""
	}
}

func (m model) viewLogin() string {
	w := max(m.width, 60)
	banner := theme.Banner(w)
	sub := theme.MutedText().Render("Same account. Same chats. From your repo.")
	emailLabel, passLabel := "email", "password"
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
	form := theme.Composer().Width(min(w-4, 56)).Render(fmt.Sprintf("%s\n%s\n\n%s\n%s\n", emailLabel, emailVal, passLabel, passVal))
	errLine := ""
	if m.err != "" {
		errLine = "\n" + theme.Error().Render(m.err)
	}
	help := theme.Footer().Render("enter sign in · g Google · tab · q")
	return lipgloss.JoinVertical(lipgloss.Left, banner, "", sub, "", form, errLine, "", help)
}

func (m model) viewChats() string {
	w := max(m.width, 60)
	banner := theme.Banner(w)
	hint := theme.MutedText().Render("Your chats · enter to open · n for new")
	sub := theme.MutedText().Render(m.status)

	var list strings.Builder
	if m.chatCreating {
		list.WriteString(theme.MutedText().Render("Creating…"))
	} else if m.chatLoad {
		list.WriteString(theme.MutedText().Render("Loading…"))
	} else {
		marker := "  "
		if m.chatIdx == 0 {
			marker = "▸ "
		}
		newRow := marker + "+ New chat"
		if m.chatIdx == 0 {
			list.WriteString(theme.Selected().Render(newRow) + "\n")
		} else {
			list.WriteString(theme.ListItem().Render(newRow) + "\n")
		}

		if len(m.chats) == 0 {
			list.WriteString("\n" + theme.MutedText().Render("No existing chats — press enter (or n) to create one."))
		} else {
			// Compact picker: never dump the whole inbox into the terminal.
			const maxVisible = 8
			start := 0
			if m.chatIdx > 0 {
				chatSel := m.chatIdx - 1
				if chatSel >= maxVisible {
					start = chatSel - maxVisible + 1
				}
			}
			end := min(len(m.chats), start+maxVisible)
			for i := start; i < end; i++ {
				chat := m.chats[i]
				pickerIdx := i + 1
				marker := "  "
				if pickerIdx == m.chatIdx {
					marker = "▸ "
				}
				num := "  "
				if i < 9 {
					num = strconv.Itoa(i+1) + "."
				}
				unread := ""
				if chat.UnreadCount > 0 {
					unread = " •"
				}
				members := strings.Join(chat.MemberNames, ", ")
				if members == "" {
					members = "Salad chat"
				}
				if len(members) > 48 {
					members = members[:45] + "…"
				}
				titleText := displayChatTitle(firstNonEmpty(chat.Title, "Untitled"))
				if len(titleText) > 52 {
					titleText = titleText[:49] + "…"
				}
				row := fmt.Sprintf("%s%s %s%s\n    %s", marker, num, titleText, unread, theme.MutedText().Render(members))
				if pickerIdx == m.chatIdx {
					list.WriteString(theme.Selected().Render(row) + "\n")
				} else {
					list.WriteString(theme.ListItem().Render(row) + "\n")
				}
			}
			if end < len(m.chats) {
				list.WriteString(theme.MutedText().Render(fmt.Sprintf("  … %d more · ↓", len(m.chats)-end)) + "\n")
			}
		}
	}
	errLine := ""
	if m.err != "" {
		errLine = theme.Error().Render(m.err) + "\n"
	}
	help := theme.Footer().Render("↑↓ · enter · n new · 1-9 · q")
	return lipgloss.JoinVertical(lipgloss.Left, banner, "", hint, sub, "", errLine+list.String(), help)
}

func (m model) viewNewAI() string {
	w := max(m.width, 60)
	banner := theme.Banner(w)
	purpose := "Who should join this chat?"
	action := "start"
	if m.aiPurpose == "add" {
		purpose = "Who else should join?"
		action = "add"
	}
	purposeLine := theme.MutedText().Render(purpose)
	count := strings.TrimSpace(m.status)
	countLine := ""
	if count != "" {
		countLine = theme.MutedText().Render(count)
	}

	var list strings.Builder
	if m.chatCreating {
		list.WriteString(theme.MutedText().Render("Working…"))
	} else if m.aiLoad {
		list.WriteString(theme.MutedText().Render("Loading…"))
	} else {
		visible := m.visibleAIProducts()
		if len(visible) == 0 {
			if m.aiPurpose == "add" {
				list.WriteString(theme.MutedText().Render("Everyone’s already here. Press m for more models."))
			} else {
				list.WriteString(theme.MutedText().Render("No AIs available."))
			}
		} else {
			for i, p := range visible {
				marker := "  "
				if i == m.aiIdx {
					marker = "▸ "
				}
				box := "[ ]"
				if m.aiSelected[p.Slug] {
					box = "[x]"
				}
				if !p.HasAccess {
					box = "[-]"
				}
				row := fmt.Sprintf("%s%s  %s", marker, box, p.DisplayName)
				if !p.HasAccess {
					row += theme.MutedText().Render("  (upgrade plan)")
				}
				// Keep rows content-width — full-width bars read like repeated footers.
				if i == m.aiIdx {
					list.WriteString(theme.Selected().Render(row) + "\n")
				} else {
					list.WriteString(theme.ListItem().Render(row) + "\n")
				}
			}
		}
	}
	errLine := ""
	if m.err != "" {
		errLine = theme.Error().Render(m.err) + "\n"
	}
	// Keys live in exactly one place.
	help := theme.Footer().Render(fmt.Sprintf("space · enter %s · a all · m more · esc", action))
	parts := []string{banner, "", purposeLine}
	if countLine != "" {
		parts = append(parts, countLine)
	}
	parts = append(parts, "", errLine+list.String(), help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewRoom() string {
	w := max(m.width, 60)
	header := theme.Header().Width(w).Render(theme.Mark() + "  ·  " + displayChatTitle(m.chatTitle))
	people := theme.MutedText().Render(participantsLine(m.members))
	body := m.viewport.View()
	mention := ""
	if m.mentionOpen {
		mention = m.renderMentionPicker(w)
	}
	composer := theme.Composer().Width(w - 2).Render(m.composer.View())
	status := strings.TrimSpace(m.status)
	if m.sending {
		status = "Sending…"
	}
	if m.err != "" {
		status = m.err
	}
	statusLine := ""
	if status != "" {
		statusLine = theme.MutedText().Render(status)
	}
	// Keep keys on their own short line so Width wrap doesn't stack a "footer wall".
	footer := theme.Footer().Render("↑↓ scroll · enter send · @mention · /add · esc")
	parts := []string{header, people, body}
	if mention != "" {
		parts = append(parts, mention)
	}
	parts = append(parts, composer)
	if statusLine != "" {
		parts = append(parts, statusLine)
	}
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) renderMentionPicker(w int) string {
	filtered := m.filteredMentions()
	if len(filtered) == 0 {
		return theme.MutedText().Render("  no matches")
	}
	var b strings.Builder
	b.WriteString(theme.MutedText().Render("  Mention") + "\n")
	limit := min(5, len(filtered))
	for i := 0; i < limit; i++ {
		mem := filtered[i]
		label := fmt.Sprintf("@%s  %s", firstNonEmpty(mem.Slug, slugify(mem.DisplayName)), mem.DisplayName)
		if strings.EqualFold(mem.MemberType, "ai") || strings.EqualFold(mem.MemberType, "app") {
			label += " · AI"
		}
		if i == m.mentionIdx {
			b.WriteString(theme.Selected().Width(min(w-4, 60)).Render(label) + "\n")
		} else {
			b.WriteString(theme.ListItem().Render(label) + "\n")
		}
	}
	return b.String()
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
		return ""
	}
	names := make([]string, 0, len(members))
	for _, mem := range members {
		if strings.EqualFold(mem.MemberType, "user") || strings.EqualFold(mem.MemberType, "human") {
			continue
		}
		label := firstNonEmpty(mem.DisplayName, mem.Slug)
		if label == "" {
			continue
		}
		names = append(names, label)
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, " · ")
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

func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
