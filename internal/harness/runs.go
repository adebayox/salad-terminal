package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/config"
)

type RunRecord struct {
	ID        string    `json:"id"`
	Workspace string    `json:"workspace"`
	Protocol  string    `json:"protocol"`
	SessionID string    `json:"session_id,omitempty"`
	Command   string    `json:"command,omitempty"`
	Config    string    `json:"config,omitempty"`
	Prompt    string    `json:"prompt"`
	StartedAt time.Time `json:"started_at"`
	ResumeOf  string    `json:"resume_of,omitempty"`
}

func SaveRun(record RunRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Workspace) == "" || strings.TrimSpace(record.Prompt) == "" {
		return errors.New("run id, workspace, and prompt are required")
	}
	dir, err := runDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, safeRunID(record.ID)+".json"), append(data, '\n'), 0o600)
}

func LoadRun(id string) (RunRecord, error) {
	id = safeRunID(id)
	if id == "" {
		return RunRecord{}, errors.New("run id is required")
	}
	dir, err := runDirectory()
	if err != nil {
		return RunRecord{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return RunRecord{}, fmt.Errorf("load harness run %q: %w", id, err)
	}
	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return RunRecord{}, fmt.Errorf("decode harness run %q: %w", id, err)
	}
	if record.ID != id || record.Workspace == "" || record.Prompt == "" {
		return RunRecord{}, errors.New("harness run record is invalid")
	}
	return record, nil
}

func runDirectory() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, installDirName, "runs"), nil
}

// SessionRoot returns a private, stable persistence directory for one trusted
// workspace. DSH's JSON-RPC runtime uses this directory to restore a named
// session after the carrier process exits; the workspace path itself is not
// used as a filename.
func SessionRoot(workspace string) (string, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return "", errors.New("workspace is required")
	}
	base, err := config.Dir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(filepath.Clean(workspace)))
	return filepath.Join(base, installDirName, "sessions", hex.EncodeToString(digest[:16])), nil
}

func EnsureSessionRoot(workspace string) (string, error) {
	root, err := SessionRoot(workspace)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("create harness session directory: %w", err)
	}
	return root, nil
}

func safeRunID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 160 || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		return ""
	}
	return id
}
