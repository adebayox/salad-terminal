package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	// Public releases use production. Set SALAD_API_URL for staging QA.
	DefaultBaseURL = "https://api.salad.ink"
	EnvBaseURL     = "SALAD_API_URL"
	EnvConfigDir   = "SALAD_CONFIG_DIR"
	keyringService = "salad-terminal"
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

func loadCredentialsFile() (*Credentials, string, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, path, err
	}
	return &creds, path, nil
}

func credentialKey(baseURL, kind string) string {
	return kind + "|" + strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func LoadCredentials() (*Credentials, error) {
	creds, path, err := loadCredentialsFile()
	if err != nil {
		return nil, err
	}
	baseURL := creds.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
		creds.BaseURL = baseURL
	}
	accessToken, accessErr := keyring.Get(keyringService, credentialKey(baseURL, "access_token"))
	refreshToken, refreshErr := keyring.Get(keyringService, credentialKey(baseURL, "refresh_token"))
	if accessErr == nil && refreshErr == nil && accessToken != "" {
		creds.AccessToken = accessToken
		creds.RefreshToken = refreshToken
		return creds, nil
	}

	// Migrate the legacy plaintext file only after both secrets are safely
	// written to the OS credential store.
	if creds.AccessToken != "" && creds.RefreshToken != "" {
		accessKey := credentialKey(baseURL, "access_token")
		refreshKey := credentialKey(baseURL, "refresh_token")
		if err := keyring.Set(keyringService, accessKey, creds.AccessToken); err != nil {
			return nil, fmt.Errorf("secure credential store unavailable: %w", err)
		}
		if err := keyring.Set(keyringService, refreshKey, creds.RefreshToken); err != nil {
			_ = keyring.Delete(keyringService, accessKey)
			return nil, fmt.Errorf("secure credential store unavailable: %w", err)
		}
		metadata := *creds
		metadata.AccessToken = ""
		metadata.RefreshToken = ""
		if err := writeCredentialsFile(path, &metadata); err != nil {
			_ = keyring.Delete(keyringService, accessKey)
			_ = keyring.Delete(keyringService, refreshKey)
			return nil, fmt.Errorf("migrate credentials file: %w", err)
		}
		return creds, nil
	}

	if accessErr != nil {
		return nil, fmt.Errorf("credentials unavailable: %w", accessErr)
	}
	return nil, fmt.Errorf("credentials unavailable: %w", refreshErr)
}

func SaveCredentials(creds *Credentials) error {
	if creds == nil || strings.TrimSpace(creds.BaseURL) == "" || creds.AccessToken == "" || creds.RefreshToken == "" {
		return errors.New("access and refresh tokens plus base URL are required")
	}
	accessKey := credentialKey(creds.BaseURL, "access_token")
	refreshKey := credentialKey(creds.BaseURL, "refresh_token")
	previousAccess, previousAccessErr := keyring.Get(keyringService, accessKey)
	previousRefresh, previousRefreshErr := keyring.Get(keyringService, refreshKey)
	restore := func() {
		if previousAccessErr == nil {
			_ = keyring.Set(keyringService, accessKey, previousAccess)
		} else {
			_ = keyring.Delete(keyringService, accessKey)
		}
		if previousRefreshErr == nil {
			_ = keyring.Set(keyringService, refreshKey, previousRefresh)
		} else {
			_ = keyring.Delete(keyringService, refreshKey)
		}
	}
	if err := keyring.Set(keyringService, accessKey, creds.AccessToken); err != nil {
		return fmt.Errorf("secure credential store unavailable: %w", err)
	}
	if err := keyring.Set(keyringService, refreshKey, creds.RefreshToken); err != nil {
		restore()
		return fmt.Errorf("secure credential store unavailable: %w", err)
	}
	metadata := *creds
	metadata.AccessToken = ""
	metadata.RefreshToken = ""
	dir, err := Dir()
	if err != nil {
		restore()
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		restore()
		return err
	}
	path, err := credentialsPath()
	if err != nil {
		restore()
		return err
	}
	if err := writeCredentialsFile(path, &metadata); err != nil {
		restore()
		return err
	}
	return nil
}

func writeCredentialsFile(path string, creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(path, data, 0o600)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".salad-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// Windows does not replace an existing file with Rename. Removing the
		// old metadata is safe here because the complete new file is already
		// durable in the same directory.
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		if retryErr := os.Rename(tmpPath, path); retryErr != nil {
			return retryErr
		}
	}
	return nil
}

func ClearCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	bases := []string{DefaultBaseURL, "https://api-staging.salad.ink"}
	if envBase := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvBaseURL)), "/"); envBase != "" {
		bases = append(bases, envBase)
	}
	if creds, _, loadErr := loadCredentialsFile(); loadErr == nil {
		if baseURL := strings.TrimRight(strings.TrimSpace(creds.BaseURL), "/"); baseURL != "" {
			bases = append(bases, baseURL)
		}
	}
	seen := map[string]bool{}
	for _, baseURL := range bases {
		if seen[baseURL] {
			continue
		}
		seen[baseURL] = true
		for _, kind := range []string{"access_token", "refresh_token"} {
			if deleteErr := keyring.Delete(keyringService, credentialKey(baseURL, kind)); deleteErr != nil && !errors.Is(deleteErr, keyring.ErrNotFound) {
				return fmt.Errorf("clear secure credentials: %w", deleteErr)
			}
		}
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
	return writeFileAtomically(path, data, 0o600)
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
	return writeFileAtomically(path, data, 0o600)
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
