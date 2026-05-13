package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// OutputConfig and DefaultsConfig are shared between GlobalConfig and
// WorkspaceConfig so the same fields appear at both levels.

type OutputConfig struct {
	DefaultFormat string `toml:"default_format"` // "table" or "json"
	Color         string `toml:"color"`          // "auto", "always", "never"
}

type DefaultsConfig struct {
	Priority int    `toml:"priority"`
	Status   string `toml:"status"`
	Type     string `toml:"type"`
	Actor    string `toml:"actor"` // "human:username"
}

// WorkspaceConfig is stored at <profile-dir>/config.toml.
// Fields left at zero value are treated as "not set" and fall back to the
// user-level values in GlobalConfig.
type WorkspaceConfig struct {
	DefaultProject string         `toml:"default_project"`
	Output         OutputConfig   `toml:"output"`
	Defaults       DefaultsConfig `toml:"defaults"`
}

// GlobalConfig is ~/.config/backlog/config.toml.
// Output and Defaults here act as user-level defaults, overridden per workspace.
type GlobalConfig struct {
	DefaultProfile string             `toml:"default_profile"`
	DefaultProject string             `toml:"default_project"`
	Profiles       map[string]Profile `toml:"profiles"`
	Output         OutputConfig       `toml:"output"`
	Defaults       DefaultsConfig     `toml:"defaults"`
}

type Profile struct {
	Path string `toml:"path"`
}

// EffectiveConfig returns the resolved config for a command invocation.
// Priority (highest first): CLI flags > workspace config > user config > hardcoded defaults.
func EffectiveConfig(global *GlobalConfig, workspace *WorkspaceConfig) *WorkspaceConfig {
	out := &WorkspaceConfig{}
	out.Output.DefaultFormat = "table"
	out.Output.Color = "auto"
	out.Defaults.Priority = 3
	out.Defaults.Status = "todo"
	out.Defaults.Type = "task"

	// Layer global (user-level) then workspace overrides. Each later layer
	// overwrites a field only when that layer set a non-zero value, so the
	// hardcoded defaults survive when nothing else supplies a value.
	apply := func(src WorkspaceConfig) {
		applyString(&out.Output.DefaultFormat, src.Output.DefaultFormat)
		applyString(&out.Output.Color, src.Output.Color)
		applyString(&out.DefaultProject, src.DefaultProject)
		applyString(&out.Defaults.Actor, src.Defaults.Actor)
		applyInt(&out.Defaults.Priority, src.Defaults.Priority)
		applyString(&out.Defaults.Status, src.Defaults.Status)
		applyString(&out.Defaults.Type, src.Defaults.Type)
	}
	apply(WorkspaceConfig{
		Output:         global.Output,
		Defaults:       global.Defaults,
		DefaultProject: global.DefaultProject,
	})
	apply(*workspace)

	return out
}

func applyString(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}

func applyInt(dst *int, v int) {
	if v != 0 {
		*dst = v
	}
}

func DefaultWorkspaceConfig() *WorkspaceConfig {
	return &WorkspaceConfig{}
}

// LoadWorkspaceConfig reads <dir>/config.toml.
// Returns an empty WorkspaceConfig (all zero values) if the file is absent —
// callers should use EffectiveConfig to apply fallback logic.
func LoadWorkspaceConfig(dir string) (*WorkspaceConfig, error) {
	c := &WorkspaceConfig{}
	path := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c, nil
	}
	if _, err := toml.DecodeFile(path, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

func WriteWorkspaceConfig(dir string, c *WorkspaceConfig) error {
	path := filepath.Join(dir, "config.toml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

// GlobalDir returns ~/.config/backlog — holds the global config file and
// profile registry.
func GlobalDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "backlog")
}

// DefaultWorkspaceRoot returns ~/.backlog — the parent directory for workspaces
// created by `backlog init` when --path is not supplied.
func DefaultWorkspaceRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".backlog")
}

// DefaultWorkspaceDir returns the default workspace path for a profile.
func DefaultWorkspaceDir(profileName string) string {
	if profileName == "" {
		profileName = "default"
	}
	return filepath.Join(DefaultWorkspaceRoot(), profileName)
}

func globalConfigPath() string {
	return filepath.Join(GlobalDir(), "config.toml")
}

func LoadGlobal() (*GlobalConfig, error) {
	c := &GlobalConfig{Profiles: map[string]Profile{}}
	path := globalConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c, nil
	}
	if _, err := toml.DecodeFile(path, c); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	return c, nil
}

func SaveGlobal(c *GlobalConfig) error {
	path := globalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func SetDefaultProject(alias string) error {
	c, err := LoadGlobal()
	if err != nil {
		return err
	}
	c.DefaultProject = alias
	return SaveGlobal(c)
}

func ClearDefaultProject() error {
	return SetDefaultProject("")
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
