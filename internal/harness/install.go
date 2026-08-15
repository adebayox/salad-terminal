package harness

import (
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/config"
)

const (
	installDirName    = "harness"
	runtimeFilePrefix = "dsh-acp-agent"
)

type installation struct {
	Runtime      string    `json:"runtime"`
	Helper       string    `json:"helper,omitempty"`
	Config       string    `json:"config,omitempty"`
	SHA256       string    `json:"sha256"`
	HelperSHA256 string    `json:"helper_sha256,omitempty"`
	InstalledAt  time.Time `json:"installed_at"`
}

func installPaths() (dir, runtimePath, manifestPath string, err error) {
	base, err := config.Dir()
	if err != nil {
		return "", "", "", err
	}
	dir = filepath.Join(base, installDirName)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	runtimePath = filepath.Join(dir, runtimeFilePrefix+ext)
	manifestPath = filepath.Join(dir, "install.json")
	return dir, runtimePath, manifestPath, nil
}

func previousInstallPaths() (runtimePath, configPath, manifestPath string, err error) {
	_, runtimePath, manifestPath, err = installPaths()
	if err != nil {
		return "", "", "", err
	}
	dir := filepath.Dir(runtimePath)
	return filepath.Join(dir, runtimeFilePrefix+".previous"), filepath.Join(dir, "cordis.yml.previous"), filepath.Join(dir, "install.previous.json"), nil
}

func managedHelperPath(runtimePath string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	return runtimePath + "-spawn-helper"
}

// InstalledCommand returns the managed ACP carrier when it exists. Explicit
// SALAD_DSH_COMMAND remains an escape hatch for development and testing.
func InstalledCommand() string {
	if value := strings.TrimSpace(os.Getenv("SALAD_DSH_COMMAND")); value != "" {
		return value
	}
	if runtimePath, _, _, installed, _ := InstallationStatus(); installed {
		return runtimePath
	}
	return "dsh-acp-demo"
}

// InstalledConfig returns the managed config path, if one was installed.
func InstalledConfig() string {
	if value := strings.TrimSpace(os.Getenv("SALAD_DSH_CONFIG")); value != "" {
		return value
	}
	_, configPath, _, installed, err := InstallationStatus()
	if err != nil || !installed || configPath == "" {
		return ""
	}
	if _, err := os.Stat(configPath); err != nil {
		return ""
	}
	return configPath
}

// Install copies a verified local carrier and optional config into Salad's
// private config directory. It never replaces an existing install unless
// force is true, and it writes the manifest only after both files are durable.
func Install(sourceRuntime, sourceConfig string, force bool) (string, error) {
	return InstallWithHelper(sourceRuntime, sourceConfig, "", force)
}

// InstallWithHelper copies a verified carrier, its optional macOS node-pty
// spawn helper, and optional config into Salad's private config directory.
// The helper is required when the carrier is a Mach-O executable on macOS.
func InstallWithHelper(sourceRuntime, sourceConfig, sourceHelper string, force bool) (string, error) {
	sourceRuntime = strings.TrimSpace(sourceRuntime)
	if sourceRuntime == "" {
		return "", errors.New("runtime path is required; pass --runtime /path/to/dsh-acp-agent")
	}
	info, err := os.Stat(sourceRuntime)
	if err != nil {
		return "", fmt.Errorf("runtime: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("runtime path must be an executable file")
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return "", errors.New("runtime file is not executable")
	}
	runtimeIsMachO := isMachOFile(sourceRuntime)
	if runtime.GOOS == "darwin" && runtimeIsMachO && strings.TrimSpace(sourceHelper) == "" {
		return "", errors.New("macOS Mach-O harness runtime requires its -spawn-helper sidecar")
	}
	if sourceHelper != "" {
		if runtime.GOOS != "darwin" {
			return "", errors.New("spawn helper is only supported for macOS harness carriers")
		}
		helperInfo, helperErr := os.Stat(sourceHelper)
		if helperErr != nil {
			return "", fmt.Errorf("spawn helper: %w", helperErr)
		}
		if helperInfo.IsDir() {
			return "", errors.New("spawn helper path must be an executable file")
		}
		if runtime.GOOS != "windows" && helperInfo.Mode()&0o111 == 0 {
			return "", errors.New("spawn helper is not executable")
		}
	}
	if sourceConfig != "" {
		configInfo, configErr := os.Stat(sourceConfig)
		if configErr != nil {
			return "", fmt.Errorf("config: %w", configErr)
		}
		if configInfo.IsDir() {
			return "", errors.New("config path must be a file")
		}
	}

	dir, runtimePath, manifestPath, err := installPaths()
	if err != nil {
		return "", err
	}
	if !force {
		if _, statErr := os.Stat(runtimePath); statErr == nil {
			return "", errors.New("a harness runtime is already installed; pass --force to replace it")
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create harness install directory: %w", err)
	}
	previousRuntime, previousConfig, previousManifest, err := previousInstallPaths()
	if err != nil {
		return "", err
	}
	previousHelper := managedHelperPath(previousRuntime)
	currentHelper := managedHelperPath(runtimePath)
	if force {
		// Keep one recoverable managed backup before replacing a release. This
		// is intentionally inside Salad's private config directory; it never
		// touches project files.
		if _, statErr := os.Stat(runtimePath); statErr == nil {
			if err := copyFileAtomic(runtimePath, previousRuntime, 0o700); err != nil {
				return "", fmt.Errorf("backup existing harness runtime: %w", err)
			}
		} else if os.IsNotExist(statErr) {
			if err := removeIfPresent(previousRuntime); err != nil {
				return "", fmt.Errorf("clear stale harness runtime backup: %w", err)
			}
		}
		if currentHelper != "" {
			if _, statErr := os.Stat(currentHelper); statErr == nil {
				if err := copyFileAtomic(currentHelper, previousHelper, 0o700); err != nil {
					return "", fmt.Errorf("backup existing harness spawn helper: %w", err)
				}
			} else if os.IsNotExist(statErr) {
				if err := removeIfPresent(previousHelper); err != nil {
					return "", fmt.Errorf("clear stale harness spawn helper backup: %w", err)
				}
			}
		}
		if _, statErr := os.Stat(manifestPath); statErr == nil {
			if err := copyFileAtomic(manifestPath, previousManifest, 0o600); err != nil {
				return "", fmt.Errorf("backup existing harness install record: %w", err)
			}
		} else if os.IsNotExist(statErr) {
			if err := removeIfPresent(previousManifest); err != nil {
				return "", fmt.Errorf("clear stale harness install backup: %w", err)
			}
		}
		currentConfig := filepath.Join(dir, "cordis.yml")
		if _, statErr := os.Stat(currentConfig); statErr == nil {
			if err := copyFileAtomic(currentConfig, previousConfig, 0o600); err != nil {
				return "", fmt.Errorf("backup existing harness config: %w", err)
			}
		} else if os.IsNotExist(statErr) {
			if err := removeIfPresent(previousConfig); err != nil {
				return "", fmt.Errorf("clear stale harness config backup: %w", err)
			}
		}
	}
	if err := copyFileAtomic(sourceRuntime, runtimePath, 0o700); err != nil {
		return "", fmt.Errorf("install runtime: %w", err)
	}
	installedHelper := ""
	if sourceHelper != "" {
		installedHelper = currentHelper
		if err := copyFileAtomic(sourceHelper, installedHelper, 0o700); err != nil {
			_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, "", manifestPath)
			return "", fmt.Errorf("install spawn helper: %w", err)
		}
	}
	if err := signMacOSMachO(runtimePath); err != nil {
		_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, "", manifestPath)
		return "", err
	}
	if err := signMacOSMachO(installedHelper); err != nil {
		_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, "", manifestPath)
		return "", err
	}

	installedConfig := ""
	if sourceConfig != "" {
		installedConfig = filepath.Join(dir, "cordis.yml")
		if err := copyFileAtomic(sourceConfig, installedConfig, 0o600); err != nil {
			_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, installedConfig, manifestPath)
			return "", fmt.Errorf("install config: %w", err)
		}
	}
	hash, err := fileSHA256(runtimePath)
	if err != nil {
		_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, installedConfig, manifestPath)
		return "", fmt.Errorf("hash installed runtime: %w", err)
	}
	helperHash := ""
	if installedHelper != "" {
		helperHash, err = fileSHA256(installedHelper)
		if err != nil {
			_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, installedConfig, manifestPath)
			return "", fmt.Errorf("hash installed spawn helper: %w", err)
		}
	}
	record := installation{Runtime: runtimePath, Helper: installedHelper, Config: installedConfig, SHA256: hash, HelperSHA256: helperHash, InstalledAt: time.Now().UTC()}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, installedConfig, manifestPath)
		return "", err
	}
	if err := writeAtomic(manifestPath, append(data, '\n'), 0o600); err != nil {
		_ = restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, installedHelper, installedConfig, manifestPath)
		return "", fmt.Errorf("write harness install record: %w", err)
	}
	return runtimePath, nil
}

// signMacOSMachO gives downloaded command-line Mach-O carriers a local
// identity before first execution. macOS can kill an unsigned downloaded
// executable before it starts; this ad-hoc signature does not claim Apple
// notarization or publisher trust, but makes the managed local carrier usable.
// Release checksums are verified before Install is called, and the manifest
// records the post-signing bytes that Salad will execute.
func signMacOSMachO(path string) error {
	if path == "" {
		return nil
	}
	if runtime.GOOS != "darwin" {
		return nil
	}
	file, err := macho.Open(path)
	if err != nil {
		return nil
	}
	_ = file.Close()

	output, err := exec.Command("codesign", "--force", "--sign", "-", path).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("sign macOS harness runtime: %w", err)
		}
		return fmt.Errorf("sign macOS harness runtime: %w: %s", err, message)
	}
	return nil
}

// Rollback restores the last managed runtime saved by a forced install. It is
// deliberately limited to Salad-owned files and never changes the workspace.
func Rollback() (string, error) {
	runtimePath, configPath, manifestPath, err := previousInstallPaths()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(runtimePath); err != nil {
		return "", errors.New("no previous harness installation is available")
	}
	_, currentRuntime, currentManifest, err := installPaths()
	if err != nil {
		return "", err
	}
	currentConfig := filepath.Join(filepath.Dir(runtimePath), "cordis.yml")
	currentHelper := managedHelperPath(currentRuntime)
	if err := restorePreviousInstallWithHelper(runtimePath, managedHelperPath(runtimePath), configPath, manifestPath, currentRuntime, currentHelper, currentConfig, currentManifest); err != nil {
		return "", fmt.Errorf("restore previous harness installation: %w", err)
	}
	return currentRuntime, nil
}

func restorePreviousInstall(previousRuntime, previousConfig, previousManifest, runtimePath, configPath, manifestPath string) error {
	return restorePreviousInstallWithHelper(previousRuntime, "", previousConfig, previousManifest, runtimePath, "", configPath, manifestPath)
}

func restorePreviousInstallWithHelper(previousRuntime, previousHelper, previousConfig, previousManifest, runtimePath, helperPath, configPath, manifestPath string) error {
	var firstErr error
	if _, err := os.Stat(previousRuntime); err == nil {
		if restoreErr := copyFileAtomic(previousRuntime, runtimePath, 0o700); restoreErr != nil {
			firstErr = restoreErr
		}
	} else if os.IsNotExist(err) {
		if removeErr := removeIfPresent(runtimePath); removeErr != nil && firstErr == nil {
			firstErr = removeErr
		}
	} else if firstErr == nil {
		firstErr = err
	}
	if helperPath != "" {
		if _, err := os.Stat(previousHelper); err == nil {
			if restoreErr := copyFileAtomic(previousHelper, helperPath, 0o700); restoreErr != nil && firstErr == nil {
				firstErr = restoreErr
			}
		} else if os.IsNotExist(err) {
			if removeErr := removeIfPresent(helperPath); removeErr != nil && firstErr == nil {
				firstErr = removeErr
			}
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if configPath != "" {
		if _, err := os.Stat(previousConfig); err == nil {
			if restoreErr := copyFileAtomic(previousConfig, configPath, 0o600); restoreErr != nil {
				if firstErr == nil {
					firstErr = restoreErr
				}
			}
		} else if os.IsNotExist(err) {
			if removeErr := removeIfPresent(configPath); removeErr != nil && firstErr == nil {
				firstErr = removeErr
			}
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if _, err := os.Stat(previousManifest); err == nil {
		if restoreErr := copyFileAtomic(previousManifest, manifestPath, 0o600); restoreErr != nil {
			if firstErr == nil {
				firstErr = restoreErr
			}
		}
	} else if os.IsNotExist(err) {
		if removeErr := removeIfPresent(manifestPath); removeErr != nil && firstErr == nil {
			firstErr = removeErr
		}
	} else if firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func removeIfPresent(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func InstallationStatus() (runtimePath, configPath, digest string, installed bool, err error) {
	_, defaultRuntime, manifestPath, err := installPaths()
	if err != nil {
		return "", "", "", false, err
	}
	data, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return defaultRuntime, "", "", false, nil
		}
		return "", "", "", false, readErr
	}
	var record installation
	if err := json.Unmarshal(data, &record); err != nil {
		return "", "", "", false, fmt.Errorf("read harness install record: %w", err)
	}
	if record.Runtime == "" {
		return "", "", "", false, errors.New("harness install record has no runtime")
	}
	if info, err := os.Stat(record.Runtime); err != nil {
		return record.Runtime, record.Config, record.SHA256, false, nil
	} else if info.IsDir() {
		return record.Runtime, record.Config, record.SHA256, false, errors.New("harness install runtime is a directory")
	}
	if record.SHA256 == "" {
		return record.Runtime, record.Config, "", false, errors.New("harness install record has no checksum")
	}
	digest, err = fileSHA256(record.Runtime)
	if err != nil {
		return record.Runtime, record.Config, "", false, fmt.Errorf("verify harness runtime: %w", err)
	}
	if !strings.EqualFold(digest, record.SHA256) {
		return record.Runtime, record.Config, digest, false, errors.New("installed harness runtime checksum mismatch")
	}
	if runtime.GOOS == "darwin" && isMachOFile(record.Runtime) {
		helperPath := record.Helper
		if helperPath == "" {
			helperPath = managedHelperPath(record.Runtime)
		}
		helperInfo, helperErr := os.Stat(helperPath)
		if helperErr != nil {
			return record.Runtime, record.Config, record.SHA256, false, fmt.Errorf("installed harness spawn helper is unavailable: %w", helperErr)
		}
		if helperInfo.IsDir() || helperInfo.Mode()&0o111 == 0 {
			return record.Runtime, record.Config, record.SHA256, false, errors.New("installed harness spawn helper is not executable")
		}
		if record.HelperSHA256 != "" {
			helperDigest, helperHashErr := fileSHA256(helperPath)
			if helperHashErr != nil {
				return record.Runtime, record.Config, record.SHA256, false, fmt.Errorf("verify harness spawn helper: %w", helperHashErr)
			}
			if !strings.EqualFold(helperDigest, record.HelperSHA256) {
				return record.Runtime, record.Config, record.SHA256, false, errors.New("installed harness spawn helper checksum mismatch")
			}
		}
	}
	return record.Runtime, record.Config, record.SHA256, true, nil
}

func isMachOFile(path string) bool {
	file, err := macho.Open(path)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFileAtomic(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".salad-harness-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := io.Copy(temporary, input); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".salad-harness-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
