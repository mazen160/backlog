package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Projects    []ManifestProject `json:"projects"`
}

type ManifestProject struct {
	Alias       string `json:"alias"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	RepoPath    string `json:"repo_path,omitempty"`
}

func Load(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "backlog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: 1}, nil
		}
		return nil, fmt.Errorf("read backlog.json: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse backlog.json: %w", err)
	}
	return &m, nil
}

func Save(dir string, m *Manifest) error {
	m.Version = 1
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "backlog.json"), data, 0644)
}
