package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	skill "github.com/mazen160/backlog/skills"
)

// skillTarget describes one AI coding tool we can install a skill into.
//
// `detect` is the directory whose existence under $HOME signals "this tool
// is installed on the user's machine". `dest` returns the absolute path the
// skill file should be written to for the given skill name, and `wrap`
// returns the final file body.
type skillTarget struct {
	tool   string
	detect string
	dest   func(home, skillName string) string
	wrap   func(s skill.Skill) string
}

func plain(s skill.Skill) string { return s.Body }

var skillTargets = []skillTarget{
	{
		tool:   "Claude Code",
		detect: ".claude",
		dest: func(home, name string) string {
			return filepath.Join(home, ".claude", "skills", name, "skill.md")
		},
		wrap: plain,
	},
	{
		tool:   "Cursor",
		detect: ".cursor",
		dest: func(home, name string) string {
			return filepath.Join(home, ".cursor", "rules", name+".mdc")
		},
		wrap: skill.CursorWrap,
	},
	{
		tool:   "OpenCode",
		detect: ".config/opencode",
		dest: func(home, name string) string {
			return filepath.Join(home, ".config", "opencode", "skills", name, "skill.md")
		},
		wrap: plain,
	},
	{
		tool:   "Codex",
		detect: ".codex",
		dest: func(home, name string) string {
			return filepath.Join(home, ".codex", "skills", name, "SKILL.md")
		},
		wrap: skill.CodexWrap,
	},
}

func newInstallSkillsCmd() *cobra.Command {
	var force, dryRun, all bool
	var only []string
	cmd := &cobra.Command{
		Use:     "install-skills",
		Aliases: []string{"install-skill"},
		Short:   "Install backlog skills into detected AI coding tools",
		Long: `Writes the embedded backlog skills into every detected AI coding tool on
this machine. By default a target is "detected" when its config directory
exists under $HOME (~/.claude, ~/.cursor, ~/.config/opencode, ~/.codex).

Use --all to install into every supported target whether or not it is
detected. Use --skill to install only specific skills (repeatable).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallSkills(only, force, dryRun, all)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing skill files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be written without writing")
	cmd.Flags().BoolVar(&all, "all", false, "install into every supported target even if not detected")
	cmd.Flags().StringSliceVar(&only, "skill", nil, "install only these skills (repeatable; default: all)")
	return cmd
}

func runInstallSkills(only []string, force, dryRun, all bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	skills, err := skill.All()
	if err != nil {
		return err
	}
	if len(only) > 0 {
		skills, err = filterSkills(skills, only)
		if err != nil {
			return err
		}
	}

	detected := 0
	for _, t := range skillTargets {
		toolDir := filepath.Join(home, t.detect)
		if !all {
			if _, statErr := os.Stat(toolDir); os.IsNotExist(statErr) {
				continue
			} else if statErr != nil {
				return fmt.Errorf("stat %s: %w", toolDir, statErr)
			}
		}
		detected++
		fmt.Printf("%s\n", t.tool)
		for _, sk := range skills {
			destFile := t.dest(home, sk.Name)
			shortPath := shortenHome(destFile, home)
			if _, statErr := os.Stat(destFile); statErr == nil && !force {
				fmt.Printf("  skip %s (already exists; --force to overwrite)\n", shortPath)
				continue
			}
			body := t.wrap(sk)
			if dryRun {
				fmt.Printf("  would write %s (%d bytes)\n", shortPath, len(body))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", filepath.Dir(destFile), err)
			}
			if err := os.WriteFile(destFile, []byte(body), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", destFile, err)
			}
			fmt.Printf("  wrote %s\n", shortPath)
		}
	}

	if detected == 0 {
		fmt.Println("No supported AI coding tools detected. Pass --all to install everywhere anyway.")
		fmt.Println("Supported targets (detected by directory presence):")
		for _, t := range skillTargets {
			fmt.Printf("  %-12s ~/%s\n", t.tool, t.detect)
		}
	}
	return nil
}

func filterSkills(all []skill.Skill, names []string) ([]skill.Skill, error) {
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}
	var out []skill.Skill
	for _, s := range all {
		if wanted[s.Name] {
			out = append(out, s)
			delete(wanted, s.Name)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for n := range wanted {
			missing = append(missing, n)
		}
		return nil, fmt.Errorf("unknown skill(s): %v", missing)
	}
	return out, nil
}

func shortenHome(path, home string) string {
	if rel, err := filepath.Rel(home, path); err == nil && !filepath.IsAbs(rel) {
		return "~/" + rel
	}
	return path
}
