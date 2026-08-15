package main

import (
	"strings"
	"testing"
)

func TestValidateEngineerNetworkMode(t *testing.T) {
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
			err := validateEngineerNetworkMode(tt.mode, tt.platform)
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
