package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL     string
	AccessToken string
	HTTP        *http.Client
}

func New(baseURL, accessToken string) *Client {
	return &Client{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		AccessToken: accessToken,
		HTTP:        &http.Client{Timeout: 45 * time.Second},
	}
}

type DeviceInfo struct {
	InstallID  string `json:"install_id"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	DeviceName string `json:"device_name"`
}

type Session struct {
	AccessToken           string    `json:"accessToken"`
	RefreshToken          string    `json:"refreshToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	UserID                string    `json:"userId"`
	InstallID             string    `json:"installId"`
	TokenType             string    `json:"tokenType"`
}

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

type AuthResponse struct {
	Session Session `json:"session"`
	User    User    `json:"user"`
}

type ChatPreview struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Preview     string    `json:"preview"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UnreadCount int       `json:"unreadCount"`
	MemberNames []string  `json:"memberNames"`
}

type BootstrapResponse struct {
	Me    User          `json:"me"`
	Chats []ChatPreview `json:"chats"`
}

type ChatMessage struct {
	ID         string    `json:"id"`
	ChatID     string    `json:"chatId"`
	Role       string    `json:"role"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	Status     string    `json:"status,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	SenderID   string    `json:"senderId,omitempty"`
}

type ChatBootstrapResponse struct {
	Chat struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Name        string   `json:"name"`
		MemberNames []string `json:"memberNames"`
	} `json:"chat"`
	Messages []ChatMessage `json:"messages"`
}

type ExplicitMention struct {
	MemberID string `json:"member_id,omitempty"`
	Token    string `json:"token,omitempty"`
	Start    int    `json:"start,omitempty"`
	End      int    `json:"end,omitempty"`
}

type OpenFileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language"`
	LineCount int    `json:"line_count"`
}

type CodeContext struct {
	SelectedCode     string            `json:"selected_code,omitempty"`
	SelectedFile     string            `json:"selected_file,omitempty"`
	Language         string            `json:"language,omitempty"`
	OpenFiles        []string          `json:"open_files,omitempty"`
	OpenFilesContent []OpenFileContent `json:"open_files_content,omitempty"`
	Diagnostics      []string          `json:"diagnostics,omitempty"`
	WorkspaceRoot    string            `json:"workspace_root,omitempty"`
}

type SendMessageRequest struct {
	Content          string            `json:"content"`
	ClientMessageID  string            `json:"client_message_id,omitempty"`
	TaggedMembers    []string          `json:"tagged_members,omitempty"`
	ExplicitMentions []ExplicitMention `json:"explicit_mentions,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	CodeContext      *CodeContext      `json:"code_context,omitempty"`
	TargetHint       *TargetHint       `json:"target_hint,omitempty"`
}

type TargetHint struct {
	MemberIDs    []string `json:"member_ids,omitempty"`
	Slugs        []string `json:"slugs,omitempty"`
	DisplayNames []string `json:"display_names,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (%d): %s", e.Code, e.Status, e.Message)
	}
	return fmt.Sprintf("http %d: %s", e.Status, e.Message)
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope struct {
			Error   string `json:"error"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(payload, &envelope)
		msg := errorText(envelope.Message, envelope.Error, strings.TrimSpace(string(payload)))
		return nil, &APIError{Status: resp.StatusCode, Code: envelope.Code, Message: msg}
	}
	return payload, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	payload, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func errorText(values ...string) string {
	if text := firstNonEmpty(values...); text != "" {
		return text
	}
	return "request failed"
}

func normalizeMessage(raw map[string]any) ChatMessage {
	sender, _ := raw["sender"].(map[string]any)
	msg := ChatMessage{
		ID:         stringField(raw, "id"),
		ChatID:     firstNonEmpty(stringField(raw, "chatId"), stringField(raw, "chat_id")),
		Role:       firstNonEmpty(stringField(raw, "role"), stringField(raw, "message_type"), stringField(raw, "sender_type")),
		AuthorName: firstNonEmpty(
			stringField(raw, "authorName"),
			stringField(raw, "sender_name"),
			stringField(sender, "display_name"),
			stringField(sender, "name"),
		),
		Body:     firstNonEmpty(stringField(raw, "body"), stringField(raw, "content")),
		Status:   firstNonEmpty(stringField(raw, "status"), stringField(raw, "message_status")),
		SenderID: firstNonEmpty(stringField(raw, "senderId"), stringField(raw, "sender_id")),
	}
	if ts := firstNonEmpty(stringField(raw, "createdAt"), stringField(raw, "created_at")); ts != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			msg.CreatedAt = parsed
		} else if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			msg.CreatedAt = parsed
		}
	}
	return msg
}

func decodeMessages(payload []byte) ([]ChatMessage, error) {
	var asArray []map[string]any
	if err := json.Unmarshal(payload, &asArray); err == nil {
		out := make([]ChatMessage, 0, len(asArray))
		for _, raw := range asArray {
			out = append(out, normalizeMessage(raw))
		}
		return out, nil
	}
	var wrapped struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(payload, &wrapped); err != nil {
		return nil, err
	}
	out := make([]ChatMessage, 0, len(wrapped.Messages))
	for _, raw := range wrapped.Messages {
		out = append(out, normalizeMessage(raw))
	}
	return out, nil
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
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

func (c *Client) Login(ctx context.Context, email, password string, device DeviceInfo) (*AuthResponse, error) {
	var out AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/mobile/auth/login", map[string]any{
		"email":       email,
		"password":    password,
		"device_info": device,
	}, &out)
	return &out, err
}

func (c *Client) LoginGoogle(ctx context.Context, code, codeVerifier, redirectURI string, device DeviceInfo) (*AuthResponse, error) {
	var out AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/mobile/auth/google", map[string]any{
		"code":          code,
		"code_verifier": codeVerifier,
		"redirect_uri":  redirectURI,
		"device_info":    device,
	}, &out)
	return &out, err
}

func (c *Client) Refresh(ctx context.Context, refreshToken string, device DeviceInfo) (*AuthResponse, error) {
	var out AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/mobile/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
		"device_info":    device,
	}, &out)
	return &out, err
}

func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/mobile/auth/logout", map[string]any{
		"refresh_token": refreshToken,
	}, nil)
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	payload, err := c.do(ctx, http.MethodGet, "/api/mobile/auth/me", nil)
	if err != nil {
		return nil, err
	}
	var direct User
	if err := json.Unmarshal(payload, &direct); err == nil && (direct.ID != "" || direct.Email != "") {
		return &direct, nil
	}
	var wrapped struct {
		User User `json:"user"`
		Me   User `json:"me"`
	}
	if err := json.Unmarshal(payload, &wrapped); err != nil {
		return nil, err
	}
	if wrapped.User.ID != "" || wrapped.User.Email != "" {
		return &wrapped.User, nil
	}
	return &wrapped.Me, nil
}

func (c *Client) Bootstrap(ctx context.Context) (*BootstrapResponse, error) {
	var out BootstrapResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/mobile/bootstrap", nil, &out)
	return &out, err
}

func (c *Client) ChatBootstrap(ctx context.Context, chatID string) (*ChatBootstrapResponse, error) {
	payload, err := c.do(ctx, http.MethodGet, "/api/mobile/chats/"+chatID+"/bootstrap", nil)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Chat struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Name        string   `json:"name"`
			MemberNames []string `json:"memberNames"`
		} `json:"chat"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	out := &ChatBootstrapResponse{}
	out.Chat.ID = raw.Chat.ID
	out.Chat.Title = raw.Chat.Title
	out.Chat.Name = raw.Chat.Name
	out.Chat.MemberNames = raw.Chat.MemberNames
	for _, msg := range raw.Messages {
		out.Messages = append(out.Messages, normalizeMessage(msg))
	}
	return out, nil
}

func (c *Client) ListMessages(ctx context.Context, chatID string) ([]ChatMessage, error) {
	payload, err := c.do(ctx, http.MethodGet, "/api/chats/"+chatID+"/messages", nil)
	if err != nil {
		return nil, err
	}
	return decodeMessages(payload)
}

func (c *Client) SendMessage(ctx context.Context, chatID, content string) (*ChatMessage, error) {
	return c.SendMessageRequest(ctx, chatID, SendMessageRequest{Content: content})
}

func (c *Client) SendMessageRequest(ctx context.Context, chatID string, req SendMessageRequest) (*ChatMessage, error) {
	if req.ClientMessageID == "" {
		req.ClientMessageID = fmt.Sprintf("term-%d", time.Now().UnixNano())
	}
	payload, err := c.do(ctx, http.MethodPost, "/api/chats/"+chatID+"/messages", req)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	if nested, ok := raw["message"].(map[string]any); ok {
		msg := normalizeMessage(nested)
		return &msg, nil
	}
	msg := normalizeMessage(raw)
	return &msg, nil
}

func (c *Client) ListMembers(ctx context.Context, chatID string) ([]map[string]any, error) {
	payload, err := c.do(ctx, http.MethodGet, "/api/chats/"+chatID+"/members", nil)
	if err != nil {
		return nil, err
	}
	var wrapped struct {
		Members []map[string]any `json:"members"`
	}
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Members != nil {
		return wrapped.Members, nil
	}
	var out []map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type AIProduct struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Provider    string `json:"provider"`
	HasAccess   bool   `json:"has_access"`
	MinimumPlan string `json:"minimum_plan,omitempty"`
}

// ListAIProducts returns the same AI catalog the web new-chat modal uses.
func (c *Client) ListAIProducts(ctx context.Context) ([]AIProduct, error) {
	payload, err := c.do(ctx, http.MethodGet, "/api/ai-products?include_locked=true", nil)
	if err != nil {
		return nil, err
	}
	var wrapped struct {
		Products []AIProduct `json:"products"`
	}
	if err := json.Unmarshal(payload, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Products, nil
}

// CreateChat creates a normal Salad chat (same POST /api/chats as the web app).
// New chats appear in Salad web immediately.
func (c *Client) CreateChat(ctx context.Context, name string, aiProductSlugs []string) (*ChatPreview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Terminal"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	if len(aiProductSlugs) == 0 {
		return nil, fmt.Errorf("pick at least one AI")
	}
	payload, err := c.do(ctx, http.MethodPost, "/api/chats", map[string]any{
		"name":             name,
		"name_source":      "salad_terminal",
		"ai_product_slugs": aiProductSlugs,
	})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	id := firstNonEmpty(stringField(raw, "id"), stringField(raw, "_id"))
	if id == "" {
		if nested, ok := raw["chat"].(map[string]any); ok {
			id = firstNonEmpty(stringField(nested, "id"), stringField(nested, "_id"))
			raw = nested
		}
	}
	if id == "" {
		return nil, fmt.Errorf("create chat: missing id in response")
	}
	title := firstNonEmpty(stringField(raw, "name"), stringField(raw, "title"), name)
	return &ChatPreview{ID: id, Title: title}, nil
}
