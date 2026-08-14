package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallVerifiesManagedRuntimeBeforeUse(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	runtimeSource := filepath.Join(t.TempDir(), "dsh-acp-agent")
	if err := os.WriteFile(runtimeSource, []byte("trusted runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	configSource := filepath.Join(t.TempDir(), "cordis.yml")
	if err := os.WriteFile(configSource, []byte("profile: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	installedPath, err := Install(runtimeSource, configSource, false)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got := InstalledCommand(); got != installedPath {
		t.Fatalf("InstalledCommand() = %q, want %q", got, installedPath)
	}
	if got := InstalledConfig(); !strings.HasSuffix(got, filepath.Join("harness", "cordis.yml")) {
		t.Fatalf("InstalledConfig() = %q", got)
	}

	if err := os.WriteFile(installedPath, []byte("tampered runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := InstalledCommand(); got != "dsh-acp-demo" {
		t.Fatalf("InstalledCommand() after tamper = %q", got)
	}
	if got := InstalledConfig(); got != "" {
		t.Fatalf("InstalledConfig() after tamper = %q", got)
	}
	_, _, _, installed, err := InstallationStatus()
	if err == nil || installed {
		t.Fatalf("InstallationStatus() = installed %v, err %v; want checksum failure", installed, err)
	}
}

func TestForcedInstallCanRollbackToPreviousRuntime(t *testing.T) {
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	first := filepath.Join(t.TempDir(), "first-agent")
	second := filepath.Join(t.TempDir(), "second-agent")
	for path, body := range map[string]string{first: "first runtime", second: "second runtime"} {
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Install(first, "", false); err != nil {
		t.Fatal(err)
	}
	installed, err := Install(second, "", true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(installed)
	if err != nil || string(data) != "second runtime" {
		t.Fatalf("after upgrade runtime = %q, err=%v", data, err)
	}
	if _, err := Rollback(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(installed)
	if err != nil || string(data) != "first runtime" {
		t.Fatalf("after rollback runtime = %q, err=%v", data, err)
	}
}
