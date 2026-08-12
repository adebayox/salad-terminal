package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func testCredentials() *Credentials {
	return &Credentials{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    "2026-08-12T00:00:00Z",
		UserID:       "user-1",
		Email:        "qa@example.test",
		Name:         "QA",
		InstallID:    "install-1",
		BaseURL:      "https://api-staging.salad.ink",
	}
}

func TestSaveCredentialsKeepsTokensOutOfDisk(t *testing.T) {
	keyring.MockInit()
	t.Setenv(EnvConfigDir, t.TempDir())
	creds := testCredentials()
	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	path, err := credentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents := string(data)
	if strings.Contains(contents, creds.AccessToken) || strings.Contains(contents, creds.RefreshToken) {
		t.Fatalf("credentials file contains secret material: %s", contents)
	}
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if loaded.AccessToken != creds.AccessToken || loaded.RefreshToken != creds.RefreshToken {
		t.Fatalf("loaded credentials lost tokens: %#v", loaded)
	}
}

func TestLoadCredentialsMigratesLegacyPlaintextFile(t *testing.T) {
	keyring.MockInit()
	t.Setenv(EnvConfigDir, t.TempDir())
	creds := testCredentials()
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials.json")
	data := `{"access_token":"access-secret","refresh_token":"refresh-secret","base_url":"https://api-staging.salad.ink","install_id":"install-1"}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() migration error = %v", err)
	}
	if loaded.AccessToken != creds.AccessToken || loaded.RefreshToken != creds.RefreshToken {
		t.Fatalf("migration lost tokens: %#v", loaded)
	}
	dataAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dataAfter), creds.AccessToken) || strings.Contains(string(dataAfter), creds.RefreshToken) {
		t.Fatalf("migration left plaintext tokens on disk: %s", dataAfter)
	}
}

func TestClearCredentialsRemovesKeyringAndMetadata(t *testing.T) {
	keyring.MockInit()
	t.Setenv(EnvConfigDir, t.TempDir())
	creds := testCredentials()
	if err := SaveCredentials(creds); err != nil {
		t.Fatal(err)
	}
	if err := ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials() error = %v", err)
	}
	if _, err := LoadCredentials(); err == nil {
		t.Fatal("LoadCredentials() succeeded after logout")
	}
}
