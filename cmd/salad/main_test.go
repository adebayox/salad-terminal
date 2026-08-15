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
