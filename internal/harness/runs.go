package harness

import (
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

func safeRunID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 160 || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		return ""
	}
	return id
}
