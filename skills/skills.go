// Package skill embeds the four agentic-loop skill directories that live
// alongside this file (backlog, backlog-enhance-tasks, backlog-loop,
// backlog-goal) into the binary and exposes them for installation into
// supported AI coding tools.
//
// The canonical source of each skill is the corresponding directory next to
// this file. Keeping the directives co-located with the markdown means a
// contributor can drop a new `<skill-name>/skill.md` in here, add a single
// embed line below, and have it picked up by `backlog install-skills`.
package skill

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed backlog/skill.md
//go:embed backlog-enhance-tasks/skill.md
//go:embed backlog-loop/skill.md
//go:embed backlog-goal/skill.md
//go:embed backlog-memory/skill.md
//go:embed backlog-memory-learn/skill.md
//go:embed backlog-memory-store/skill.md
var fsys embed.FS

// Skill is one named skill with its markdown body.
type Skill struct {
	Name string
	Body string
}

// All returns every embedded skill, sorted with the base "backlog" skill
// first and the workflow skills after it.
func All() ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		body, err := fs.ReadFile(fsys, e.Name()+"/skill.md")
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", e.Name(), err)
		}
		skills = append(skills, Skill{Name: e.Name(), Body: string(body)})
	}
	sort.Slice(skills, func(i, j int) bool {
		// Keep the base "backlog" skill first; everything else stays alphabetical.
		if skills[i].Name == "backlog" {
			return true
		}
		if skills[j].Name == "backlog" {
			return false
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

// Get returns a single skill by name.
func Get(name string) (Skill, error) {
	body, err := fs.ReadFile(fsys, name+"/skill.md")
	if err != nil {
		return Skill{}, fmt.Errorf("skill %q not found", name)
	}
	return Skill{Name: name, Body: string(body)}, nil
}

// Names returns every embedded skill name.
func Names() ([]string, error) {
	skills, err := All()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name
	}
	return out, nil
}

// CursorWrap converts a raw skill body into Cursor's .mdc rule format. If
// the skill body already starts with a YAML frontmatter block we strip it,
// pull the description out of it, and re-emit a Cursor-shaped frontmatter
// in front of the remaining body.
func CursorWrap(s Skill) string {
	desc, body := splitFrontmatter(s.Body)
	if desc == "" {
		desc = fmt.Sprintf("Backlog %s skill", s.Name)
	}
	if len(desc) > 200 {
		desc = desc[:197] + "..."
	}
	return "---\ndescription: " + desc + "\nglobs:\nalwaysApply: false\n---\n\n" + body
}

// splitFrontmatter returns (description, body-without-frontmatter). If the
// input does not begin with a `---` frontmatter block the input is returned
// unchanged as the body and the description is empty.
func splitFrontmatter(s string) (string, string) {
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return "", s
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(s, "---\n"), "---\r\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", s
	}
	frontmatter := rest[:end]
	body := strings.TrimLeft(rest[end+len("\n---"):], "\r\n")

	var desc string
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			break
		}
	}
	return desc, body
}
