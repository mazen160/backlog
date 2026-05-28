package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInstallSkillsWritesCodexSkillDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := runInstallSkills([]string{"backlog"}, false, false, true); err != nil {
		t.Fatalf("runInstallSkills: %v", err)
	}

	dest := filepath.Join(home, ".codex", "skills", "backlog", "SKILL.md")
	body, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read Codex skill: %v", err)
	}

	got := string(body)
	for _, want := range []string{
		"---\nname: backlog\n",
		"description: Interact with the Backlog CLI",
		"You have access to the `backlog` CLI.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Codex skill body missing %q", want)
		}
	}

	oldPromptPath := filepath.Join(home, ".codex", "prompts", "backlog.md")
	if _, err := os.Stat(oldPromptPath); !os.IsNotExist(err) {
		t.Fatalf("old Codex prompt path exists or stat failed unexpectedly: %v", err)
	}
}
