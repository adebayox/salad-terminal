package main

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateHarnessNetworkMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		platform string
		wantErr  string
	}{
		{name: "deny linux", mode: "deny", platform: "linux"},
		{name: "allow linux", mode: "allow", platform: "linux"},
		{name: "loopback macos", mode: "loopback", platform: "darwin"},
		{name: "unknown mode", mode: "internet", platform: "darwin", wantErr: "unsupported harness network mode"},
		{name: "loopback linux", mode: "loopback", platform: "linux", wantErr: "Linux does not support --network loopback yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHarnessNetworkMode(tt.mode, tt.platform)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateEngineerNetworkMode() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateEngineerNetworkMode() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestHarnessReceiptChatIDIsExplicitOptIn(t *testing.T) {
	if got := harnessReceiptChatID("", ""); got != "" {
		t.Fatalf("implicit receipt chat = %q, want empty", got)
	}
	if got := harnessReceiptChatID("", "env-chat"); got != "env-chat" {
		t.Fatalf("environment receipt chat = %q, want env-chat", got)
	}
	if got := harnessReceiptChatID("flag-chat", "env-chat"); got != "flag-chat" {
		t.Fatalf("explicit receipt chat = %q, want flag-chat", got)
	}
}

func TestHarnessProviderRecoveryMessageOnlyForProviderFailures(t *testing.T) {
	cases := []struct {
		name     string
		err      string
		provider string
		want     string
	}{
		{name: "default provider", err: "ACP turn failed: DeepSeek API error (HTTP 502)", want: "default model provider failed"},
		{name: "selected provider", err: "provider request failed", provider: "xai", want: `provider "xai" failed`},
		{name: "unrelated error", err: "workspace not trusted", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The helper writes to stderr; the assertion is covered by the
			// branch-specific message construction below without changing the
			// user's terminal output stream in this unit test.
			message := harnessProviderRecoveryMessage("salad engineer", tc.provider, errors.New(tc.err))
			if tc.want == "" {
				if message != "" {
					t.Fatalf("message = %q, want empty", message)
				}
				return
			}
			if !strings.Contains(message, tc.want) {
				t.Fatalf("message = %q, want substring %q", message, tc.want)
			}
		})
	}
}

func TestEngineerCommandIsNotASecondTerminal(t *testing.T) {
	err := run([]string{"engineer"})
	if err == nil || !strings.Contains(err.Error(), "no longer a separate terminal") {
		t.Fatalf("run(engineer) error = %v", err)
	}
}
