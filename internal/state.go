package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StateRecord struct {
	App           string `json:"app"`
	PreviousImage string `json:"previous_image"`
	DeployedImage string `json:"deployed_image"`
	DeployedAt    string `json:"deployed_at"`
	LastGoodImage string `json:"last_good_image,omitempty"`
	LastGoodAt    string `json:"last_good_at,omitempty"`
}

func (s StateRecord) RollbackImage() string {
	if image := strings.TrimSpace(s.LastGoodImage); image != "" {
		return image
	}
	if image := strings.TrimSpace(s.PreviousImage); image != "" {
		return image
	}
	return strings.TrimSpace(s.DeployedImage)
}

func StateFilePath(app string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "containers", "systemd", app+".state.json"), nil
}

func ReadState(path string) (*StateRecord, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}

	var state StateRecord
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}

	return &state, nil
}

func WriteStateAtomic(path string, state StateRecord) error {
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state file %s: %w", path, err)
	}
	payload = append(payload, '\n')

	if err := AtomicWriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write state file %s: %w", path, err)
	}

	return nil
}
