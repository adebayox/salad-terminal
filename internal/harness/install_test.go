package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	secondConfig := filepath.Join(t.TempDir(), "second-cordis.yml")
	for path, body := range map[string]string{first: "first runtime", second: "second runtime"} {
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(secondConfig, []byte("profile: second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(first, "", false); err != nil {
		t.Fatal(err)
	}
	installed, err := Install(second, secondConfig, true)
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
	if _, err := os.Stat(filepath.Join(filepath.Dir(installed), "cordis.yml")); !os.IsNotExist(err) {
		t.Fatalf("after rollback stale config exists: %v", err)
	}
}

func TestRestorePreviousInstallRemovesPartialFirstInstall(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "dsh-acp-agent")
	configPath := filepath.Join(dir, "cordis.yml")
	manifestPath := filepath.Join(dir, "install.json")
	for _, path := range []string{runtimePath, configPath, manifestPath} {
		if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := restorePreviousInstall(
		filepath.Join(dir, "missing-runtime"),
		filepath.Join(dir, "missing-config"),
		filepath.Join(dir, "missing-manifest"),
		runtimePath, configPath, manifestPath,
	); err != nil {
		t.Fatalf("restorePreviousInstall() error = %v", err)
	}
	for _, path := range []string{runtimePath, configPath, manifestPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("partial managed file still exists at %s: %v", path, err)
		}
	}
}

func TestInstallSignsMacOSMachORuntime(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only carrier execution contract")
	}
	t.Setenv("SALAD_CONFIG_DIR", t.TempDir())
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	installed, err := InstallWithHelper(source, "", source, false)
	if err != nil {
		t.Fatalf("InstallWithHelper() error = %v", err)
	}
	if err := exec.Command("codesign", "--verify", "--verbose", installed).Run(); err != nil {
		t.Fatalf("installed Mach-O is not executable under macOS signing policy: %v", err)
	}
	helper := installed + "-spawn-helper"
	if _, err := os.Stat(helper); err != nil {
		t.Fatalf("installed spawn helper missing: %v", err)
	}
	if _, _, _, installed, err := InstallationStatus(); err != nil || !installed {
		t.Fatalf("InstallationStatus() = installed %v, err %v; want valid helper install", installed, err)
	}
}
