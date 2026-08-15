package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/salad-ai/salad-terminal/internal/config"
)

type RunRecord struct {
	ID         string    `json:"id"`
	Workspace  string    `json:"workspace"`
	Protocol   string    `json:"protocol"`
	SessionID  string    `json:"session_id,omitempty"`
	Command    string    `json:"command,omitempty"`
	Config     string    `json:"config,omitempty"`
	Prompt     string    `json:"prompt"`
	StartedAt  time.Time `json:"started_at"`
	ResumeOf   string    `json:"resume_of,omitempty"`
	Status     string    `json:"status,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
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

// ListRuns returns saved local run records, newest first. A corrupt record is
// reported instead of silently disappearing; a missing or damaged record can
// affect whether a developer resumes the right workspace.
func ListRuns() ([]RunRecord, error) {
	dir, err := runDirectory()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunRecord{}, nil
		}
		return nil, fmt.Errorf("list harness runs: %w", err)
	}
	runs := make([]RunRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read harness run %q: %w", entry.Name(), readErr)
		}
		var record RunRecord
		if decodeErr := json.Unmarshal(data, &record); decodeErr != nil {
			return nil, fmt.Errorf("decode harness run %q: %w", entry.Name(), decodeErr)
		}
		if safeRunID(record.ID) == "" || record.Workspace == "" || record.Prompt == "" {
			return nil, fmt.Errorf("harness run %q is invalid", entry.Name())
		}
		runs = append(runs, record)
	}
	sort.SliceStable(runs, func(i, j int) bool {
		left := runs[i].UpdatedAt
		if left.IsZero() {
			left = runs[i].StartedAt
		}
		right := runs[j].UpdatedAt
		if right.IsZero() {
			right = runs[j].StartedAt
		}
		return left.After(right)
	})
	return runs, nil
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
