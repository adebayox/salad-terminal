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
	AvatarURL string `json:"avatar_url"`
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
	CreatedAt  time.Time `json:"createdAt"`
	SenderID   string    `json:"senderId,omitempty"`
}

type ChatBootstrapResponse struct {
	Chat struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		MemberNames []string `json:"memberNames"`
	} `json:"chat"`
	Messages []ChatMessage `json:"messages"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
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

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
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
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope struct {
			Error   string `json:"error"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(payload, &envelope)
		msg := firstNonEmpty(envelope.Message, envelope.Error, strings.TrimSpace(string(payload)))
		return &APIError{Status: resp.StatusCode, Code: envelope.Code, Message: msg}
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
	return "request failed"
}

func (c *Client) Login(ctx context.Context, email, password string, device DeviceInfo) (*AuthResponse, error) {
	var out AuthResponse
	err := c.do(ctx, http.MethodPost, "/api/mobile/auth/login", map[string]any{
		"email":       email,
		"password":    password,
		"device_info": device,
	}, &out)
	return &out, err
}

func (c *Client) Refresh(ctx context.Context, refreshToken string, device DeviceInfo) (*AuthResponse, error) {
	var out AuthResponse
	err := c.do(ctx, http.MethodPost, "/api/mobile/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
		"device_info":    device,
	}, &out)
	return &out, err
}

func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	return c.do(ctx, http.MethodPost, "/api/mobile/auth/logout", map[string]any{
		"refresh_token": refreshToken,
	}, nil)
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
		Me   User `json:"me"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/mobile/auth/me", nil, &out); err != nil {
		return nil, err
	}
	if out.User.ID != "" {
		return &out.User, nil
	}
	if out.Me.ID != "" {
		return &out.Me, nil
	}
	// Some deployments return the user object at the top level.
	var direct User
	if err := c.do(ctx, http.MethodGet, "/api/mobile/auth/me", nil, &direct); err == nil && direct.ID != "" {
		return &direct, nil
	}
	return &out.User, nil
}

func (c *Client) Bootstrap(ctx context.Context) (*BootstrapResponse, error) {
	var out BootstrapResponse
	err := c.do(ctx, http.MethodGet, "/api/mobile/bootstrap", nil, &out)
	return &out, err
}

func (c *Client) ChatBootstrap(ctx context.Context, chatID string) (*ChatBootstrapResponse, error) {
	var out ChatBootstrapResponse
	err := c.do(ctx, http.MethodGet, "/api/mobile/chats/"+chatID+"/bootstrap", nil, &out)
	return &out, err
}

func (c *Client) ListMessages(ctx context.Context, chatID string) ([]ChatMessage, error) {
	var out []ChatMessage
	var wrapped struct {
		Messages []ChatMessage `json:"messages"`
	}
	path := "/api/chats/" + chatID + "/messages"
	if err := c.do(ctx, http.MethodGet, path, nil, &wrapped); err == nil && len(wrapped.Messages) > 0 {
		return wrapped.Messages, nil
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID, content string) (*ChatMessage, error) {
	var out ChatMessage
	var wrapped struct {
		Message ChatMessage `json:"message"`
	}
	body := SendMessageRequest{Content: content}
	path := "/api/chats/" + chatID + "/messages"
	if err := c.do(ctx, http.MethodPost, path, body, &wrapped); err == nil && wrapped.Message.ID != "" {
		return &wrapped.Message, nil
	}
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListMembers(ctx context.Context, chatID string) ([]map[string]any, error) {
	var wrapped struct {
		Members []map[string]any `json:"members"`
	}
	path := "/api/chats/" + chatID + "/members"
	if err := c.do(ctx, http.MethodGet, path, nil, &wrapped); err == nil {
		return wrapped.Members, nil
	}
	var out []map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
