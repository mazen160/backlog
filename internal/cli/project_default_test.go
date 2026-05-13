package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestProjectOrDefault(t *testing.T) {
	prev := app.DefaultProject
	t.Cleanup(func() { app.DefaultProject = prev })
	app.DefaultProject = "backlog"

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("project", "", "")

	if got := projectOrDefault(cmd, ""); got != "backlog" {
		t.Fatalf("projectOrDefault without flag = %q, want backlog", got)
	}
	if got := projectOrDefault(cmd, "explicit"); got != "explicit" {
		t.Fatalf("projectOrDefault with value = %q, want explicit", got)
	}
	if err := cmd.Flags().Set("project", ""); err != nil {
		t.Fatalf("set project flag: %v", err)
	}
	if got := projectOrDefault(cmd, ""); got != "" {
		t.Fatalf("projectOrDefault with explicit empty flag = %q, want empty", got)
	}
}

func TestRequireProjectOrDefault(t *testing.T) {
	prev := app.DefaultProject
	t.Cleanup(func() { app.DefaultProject = prev })
	app.DefaultProject = ""

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("project", "", "")
	if _, err := requireProjectOrDefault(cmd, ""); err == nil {
		t.Fatalf("requireProjectOrDefault without project/default succeeded, want error")
	}

	app.DefaultProject = "backlog"
	got, err := requireProjectOrDefault(cmd, "")
	if err != nil {
		t.Fatalf("requireProjectOrDefault with default: %v", err)
	}
	if got != "backlog" {
		t.Fatalf("requireProjectOrDefault = %q, want backlog", got)
	}
}

func TestProjectDefaultCommandsMatchProfileVocabulary(t *testing.T) {
	cmd := newProjectCmd()

	for _, name := range []string{"set-default", "clear-default", "use", "current"} {
		if got, _, err := cmd.Find([]string{name}); err != nil || got == nil || got.Name() != name {
			t.Fatalf("project command %q missing: got=%v err=%v", name, got, err)
		}
	}

	for _, name := range []string{"default"} {
		if got, _, err := cmd.Find([]string{name}); err == nil && got != nil && got.Name() == name {
			t.Fatalf("project command %q should not exist", name)
		}
	}
}
