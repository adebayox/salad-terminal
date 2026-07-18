package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/config"
)

const (
	versionURL = "https://github.com/adebayox/salad-terminal/releases/download/latest/VERSION"
	repoAPI    = "https://api.github.com/repos/adebayox/salad-terminal/commits/main"
	installURL = "https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh"
	envDisable = "SALAD_DISABLE_AUTOUPDATER"
)

type state struct {
	CheckedAt  time.Time `json:"checked_at"`
	RemoteSHA  string    `json:"remote_sha,omitempty"`
	AppliedSHA string    `json:"applied_sha,omitempty"`
}

// MaybeAutoUpdate checks GitHub main and reinstalls when behind.
// Matches Claude Code / Codex: auto on launch, opt out with SALAD_DISABLE_AUTOUPDATER=1.
// Returns true when the binary was replaced and the process should re-exec.
func MaybeAutoUpdate(currentVersion string) (updated bool) {
	if os.Getenv(envDisable) != "" {
		return false
	}
	if os.Getenv("CI") != "" || os.Getenv("SALAD_CI") != "" {
		return false
	}
	// Avoid recursive update loops after re-exec.
	if os.Getenv("SALAD_SKIP_AUTOUPDATE") == "1" {
		return false
	}
	// Contributors hacking on the repo: don't overwrite a local build.
	if isLocalDevCheckout() {
		return false
	}

	current := normalizeSHA(currentVersion)
	st := loadState()

	remote, err := fetchMainSHA()
	if err != nil || remote == "" {
		return false
	}
	st.CheckedAt = time.Now().UTC()
	st.RemoteSHA = remote
	_ = saveState(st)

	if current != "" && current == remote {
		return false
	}

	fmt.Fprintf(os.Stderr, "Updating Salad Terminal %s → %s…\n", displayVer(current), remote)
	if err := runInstall(); err != nil {
		fmt.Fprintf(os.Stderr, "auto-update failed: %v (run: salad update)\n", err)
		return false
	}
	st.AppliedSHA = remote
	_ = saveState(st)
	fmt.Fprintf(os.Stderr, "Updated to %s\n", remote)
	return true
}

// Reexec runs the current binary again with the same args (after an update).
func Reexec() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	env := append([]string{}, os.Environ()...)
	env = append(env, "SALAD_SKIP_AUTOUPDATE=1")
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

func fetchMainSHA() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Prefer the published release VERSION (what install.sh actually ships).
	if sha, err := fetchReleaseVersion(ctx); err == nil && sha != "" {
		return sha, nil
	}

	// Fallback: git ls-remote (no API rate limit).
	if sha, err := gitLSRemoteSHA(ctx); err == nil && sha != "" {
		return sha, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "salad-terminal")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github %s", resp.Status)
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return normalizeSHA(payload.SHA), nil
}

func fetchReleaseVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "salad-terminal")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version %s", resp.Status)
	}
	var buf [64]byte
	n, _ := resp.Body.Read(buf[:])
	return normalizeSHA(string(buf[:n])), nil
}

func gitLSRemoteSHA(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "https://github.com/adebayox/salad-terminal.git", "refs/heads/main")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty ls-remote")
	}
	return normalizeSHA(fields[0]), nil
}

func runInstall() error {
	// Binary install (no Go). SALAD_FORCE_REMOTE keeps contributor checkouts from
	// rebuilding local source during auto-update.
	cmd := exec.Command("bash", "-c", "curl -fsSL "+installURL+" | SALAD_FORCE_REMOTE=1 bash")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func normalizeSHA(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" || v == "dev" {
		return ""
	}
	if len(v) > 7 {
		return v[:7]
	}
	return v
}

func displayVer(v string) string {
	if v == "" {
		return "(unknown)"
	}
	return v
}

func statePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update_state.json"), nil
}

func loadState() state {
	path, err := statePath()
	if err != nil {
		return state{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state{}
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return state{}
	}
	return st
}

func saveState(st state) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LatestSHA is exported for diagnostics.
func LatestSHA() (string, error) {
	return fetchMainSHA()
}

func isLocalDevCheckout() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe, _ = filepath.EvalSymlinks(exe)
	dir := filepath.Dir(exe)
	// Built as ./salad inside the repo, or go run from module root.
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			data, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
			if strings.Contains(string(data), "salad-terminal") {
				if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
					return true
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}
