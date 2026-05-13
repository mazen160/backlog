package profile

import (
	"fmt"
	"path/filepath"

	"github.com/mazen160/backlog/internal/config"
)

// Resolve returns the workspace directory for a named profile.
// If name is empty, returns the default profile path (if configured).
func Resolve(name string) (string, error) {
	g, err := config.LoadGlobal()
	if err != nil {
		return "", err
	}
	if name == "" {
		name = g.DefaultProfile
	}
	if name == "" {
		return "", nil // caller will fall back to cwd walk
	}
	p, ok := g.Profiles[name]
	if !ok {
		return "", fmt.Errorf("profile %q not found", name)
	}
	return config.ExpandPath(p.Path), nil
}

// Add registers a new profile. Returns an error if the name already exists.
func Add(name, path string) error {
	g, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if _, exists := g.Profiles[name]; exists {
		return fmt.Errorf("profile %q already exists", name)
	}
	g.Profiles[name] = config.Profile{Path: filepath.Clean(path)}
	return config.SaveGlobal(g)
}

// Upsert registers a profile, overwriting any existing entry with the same name.
func Upsert(name, path string) error {
	g, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	g.Profiles[name] = config.Profile{Path: filepath.Clean(path)}
	return config.SaveGlobal(g)
}

// HasDefault reports whether a default profile is configured.
func HasDefault() (bool, error) {
	g, err := config.LoadGlobal()
	if err != nil {
		return false, err
	}
	return g.DefaultProfile != "", nil
}

// Remove deletes a profile from the registry (does not touch the DB).
func Remove(name string) error {
	g, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if _, ok := g.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(g.Profiles, name)
	return config.SaveGlobal(g)
}

// ClearDefault removes the active default profile without deleting any profile entry.
func ClearDefault() error {
	g, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	g.DefaultProfile = ""
	return config.SaveGlobal(g)
}

// SetDefault sets the default profile.
func SetDefault(name string) error {
	g, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if _, ok := g.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	g.DefaultProfile = name
	return config.SaveGlobal(g)
}

// List returns all registered profiles.
func List() (map[string]config.Profile, string, error) {
	g, err := config.LoadGlobal()
	if err != nil {
		return nil, "", err
	}
	return g.Profiles, g.DefaultProfile, nil
}
