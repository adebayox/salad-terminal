package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Staging until Terminal collaboration matrix passes (see TERMINAL_CONTRACT.md).
	DefaultBaseURL = "https://api-staging.salad.ink"
	EnvBaseURL     = "SALAD_API_URL"
	EnvConfigDir   = "SALAD_CONFIG_DIR"
)

type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	InstallID    string `json:"install_id"`
	BaseURL      string `json:"base_url,omitempty"`
}

type ActiveChat struct {
	ChatID string `json:"chat_id"`
	Title  string `json:"title,omitempty"`
}

// WorkspaceBinding ties a local repo path to a Salad chat (Claude Code / Codex style continue).
type WorkspaceBinding struct {
	ChatID string `json:"chat_id"`
	Title  string `json:"title,omitempty"`
}

type WorkspaceBindings map[string]WorkspaceBinding

func Dir() (string, error) {
	if override := os.Getenv(EnvConfigDir); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "salad-terminal"), nil
	case "windows":
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, "salad-terminal"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "salad-terminal"), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "salad-terminal"), nil
		}
		return filepath.Join(home, ".config", "salad-terminal"), nil
	}
}

func BaseURL() string {
	if v := os.Getenv(EnvBaseURL); v != "" {
		return v
	}
	creds, err := LoadCredentials()
	if err == nil && creds.BaseURL != "" {
		return creds.BaseURL
	}
	return DefaultBaseURL
}

func credentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func activeChatPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "active_chat.json"), nil
}

func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	if creds.AccessToken == "" {
		return nil, errors.New("not logged in")
	}
	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func ClearCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func LoadActiveChat() (*ActiveChat, error) {
	path, err := activeChatPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var active ActiveChat
	if err := json.Unmarshal(data, &active); err != nil {
		return nil, err
	}
	if active.ChatID == "" {
		return nil, errors.New("no active chat")
	}
	return &active, nil
}

func SaveActiveChat(active *ActiveChat) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := activeChatPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func workspaceBindingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "workspace_chats.json"), nil
}

func LoadWorkspaceBindings() (WorkspaceBindings, error) {
	path, err := workspaceBindingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceBindings{}, nil
		}
		return nil, err
	}
	var bindings WorkspaceBindings
	if err := json.Unmarshal(data, &bindings); err != nil {
		return nil, err
	}
	if bindings == nil {
		bindings = WorkspaceBindings{}
	}
	return bindings, nil
}

func SaveWorkspaceBindings(bindings WorkspaceBindings) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := workspaceBindingsPath()
	if err != nil {
		return err
	}
	if bindings == nil {
		bindings = WorkspaceBindings{}
	}
	data, err := json.MarshalIndent(bindings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// BindWorkspace remembers which Salad chat belongs to this local folder (for `salad` continue).
func BindWorkspace(workspaceRoot, chatID, title string) error {
	root := filepath.Clean(workspaceRoot)
	if root == "" || chatID == "" {
		return errors.New("workspace root and chat id required")
	}
	bindings, err := LoadWorkspaceBindings()
	if err != nil {
		return err
	}
	bindings[root] = WorkspaceBinding{ChatID: chatID, Title: title}
	if err := SaveWorkspaceBindings(bindings); err != nil {
		return err
	}
	return SaveActiveChat(&ActiveChat{ChatID: chatID, Title: title})
}

func WorkspaceChatID(workspaceRoot string) (string, string, error) {
	root := filepath.Clean(workspaceRoot)
	bindings, err := LoadWorkspaceBindings()
	if err != nil {
		return "", "", err
	}
	if b, ok := bindings[root]; ok && b.ChatID != "" {
		return b.ChatID, b.Title, nil
	}
	return "", "", errors.New("no workspace chat")
}
