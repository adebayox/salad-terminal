package auth

import (
	"strings"
	"testing"

	"github.com/salad-ai/salad-terminal/internal/config"
)

func TestDeviceInfoIdentifiesTerminalSurface(t *testing.T) {
	info := DeviceInfo("install-test")
	if info.InstallID != "install-test" {
		t.Fatalf("InstallID = %q, want install-test", info.InstallID)
	}
	if info.Platform != "terminal" {
		t.Fatalf("Platform = %q, want terminal", info.Platform)
	}
	if info.AppVersion == "" || info.DeviceName == "" {
		t.Fatal("terminal device metadata must include version and name")
	}
}

func TestAuthenticatedBaseURLRejectsEnvironmentMixup(t *testing.T) {
	t.Setenv(config.EnvBaseURL, "https://api.salad.ink")
	_, err := authenticatedBaseURL(&config.Credentials{BaseURL: "https://api-staging.salad.ink"})
	if err == nil || !strings.Contains(err.Error(), "credentials belong to https://api-staging.salad.ink") {
		t.Fatalf("authenticatedBaseURL() error = %v", err)
	}
}

func TestAuthenticatedBaseURLUsesStoredEnvironment(t *testing.T) {
	t.Setenv(config.EnvBaseURL, "https://api-staging.salad.ink/")
	base, err := authenticatedBaseURL(&config.Credentials{BaseURL: "https://api-staging.salad.ink/"})
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://api-staging.salad.ink" {
		t.Fatalf("base URL = %q", base)
	}
}

func TestWebAuthURLKeepsStagingFlowsInStaging(t *testing.T) {
	if got := webAuthURL("https://api-staging.salad.ink", "signup"); got != "https://staging.salad.ink/?auth=signup" {
		t.Fatalf("staging signup URL = %q", got)
	}
	if got := webAuthURL("https://api.salad.ink", "forgot"); got != "https://salad.ink/?auth=forgot" {
		t.Fatalf("production recovery URL = %q", got)
	}
}
