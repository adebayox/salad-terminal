package auth

import "testing"

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
