package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultWorkspaceDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := DefaultWorkspaceDir("default")
	want := filepath.Join(home, ".backlog", "default")
	if got != want {
		t.Fatalf("DefaultWorkspaceDir(default) = %q, want %q", got, want)
	}

	got = DefaultWorkspaceDir("")
	if got != want {
		t.Fatalf("DefaultWorkspaceDir(empty) = %q, want %q", got, want)
	}
}

func TestEffectiveConfigDefaultProject(t *testing.T) {
	got := EffectiveConfig(
		&GlobalConfig{DefaultProject: "global"},
		&WorkspaceConfig{},
	)
	if got.DefaultProject != "global" {
		t.Fatalf("DefaultProject from global = %q, want global", got.DefaultProject)
	}

	got = EffectiveConfig(
		&GlobalConfig{DefaultProject: "global"},
		&WorkspaceConfig{DefaultProject: "workspace"},
	)
	if got.DefaultProject != "workspace" {
		t.Fatalf("DefaultProject from workspace = %q, want workspace", got.DefaultProject)
	}
}

func TestSetDefaultProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := SetDefaultProject("backlog"); err != nil {
		t.Fatalf("SetDefaultProject: %v", err)
	}
	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if cfg.DefaultProject != "backlog" {
		t.Fatalf("DefaultProject = %q, want backlog", cfg.DefaultProject)
	}

	if err := ClearDefaultProject(); err != nil {
		t.Fatalf("ClearDefaultProject: %v", err)
	}
	cfg, err = LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal after clear: %v", err)
	}
	if cfg.DefaultProject != "" {
		t.Fatalf("DefaultProject after clear = %q, want empty", cfg.DefaultProject)
	}

	if _, err := os.Stat(filepath.Join(home, ".config", "backlog", "config.toml")); err != nil {
		t.Fatalf("global config was not written: %v", err)
	}
}
