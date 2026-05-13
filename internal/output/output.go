package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/mazen160/backlog/internal/models"
)

// Printer abstracts JSON vs human-readable output.
type Printer struct {
	w     io.Writer
	json  bool
	quiet bool
}

func New(jsonMode bool, quiet ...bool) *Printer {
	q := false
	if len(quiet) > 0 {
		q = quiet[0]
	}
	return &Printer{w: os.Stdout, json: jsonMode, quiet: q}
}

func (p *Printer) JSON() bool { return p.json }

// --- Generic JSON / text helpers ---

func (p *Printer) PrintJSON(v interface{}) {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

func (p *Printer) Println(s string) {
	fmt.Fprintln(p.w, s)
}

func (p *Printer) Success(s string) {
	if !p.json && !p.quiet {
		fmt.Fprintln(p.w, color.GreenString("✓")+" "+s)
	}
}

func (p *Printer) CommandResult(action, entity, id, ref, message string) {
	if p.json {
		p.PrintJSON(struct {
			OK      bool   `json:"ok"`
			Action  string `json:"action,omitempty"`
			Entity  string `json:"entity,omitempty"`
			ID      string `json:"id,omitempty"`
			Ref     string `json:"ref,omitempty"`
			Message string `json:"message"`
		}{
			OK:      true,
			Action:  action,
			Entity:  entity,
			ID:      id,
			Ref:     ref,
			Message: message,
		})
		return
	}
	p.Success(message)
}

func (p *Printer) Error(s string) {
	fmt.Fprintln(os.Stderr, color.RedString("✗")+" "+s)
}

// --- Projects ---

func (p *Printer) Projects(projects []*models.Project) {
	p.ProjectsWithDefault(projects, "")
}

func (p *Printer) ProjectsWithDefault(projects []*models.Project, defaultAlias string) {
	if projects == nil {
		projects = []*models.Project{}
	}
	if p.json {
		payload := map[string]interface{}{"projects": projects}
		if defaultAlias != "" {
			payload["default_project"] = defaultAlias
		}
		p.PrintJSON(payload)
		return
	}
	t := newTable(p.w)
	if defaultAlias != "" {
		t.AppendHeader(table.Row{"", "ALIAS", "NAME", "REPO", "CREATED"})
	} else {
		t.AppendHeader(table.Row{"ALIAS", "NAME", "REPO", "CREATED"})
	}
	for _, pr := range projects {
		if defaultAlias != "" {
			marker := " "
			if pr.Alias == defaultAlias {
				marker = "*"
			}
			t.AppendRow(table.Row{marker, pr.Alias, pr.Name, pr.RepoPath, formatTime(pr.CreatedAt)})
			continue
		}
		t.AppendRow(table.Row{pr.Alias, pr.Name, pr.RepoPath, formatTime(pr.CreatedAt)})
	}
	t.Render()
}

func (p *Printer) Project(pr *models.Project) {
	if p.json {
		p.PrintJSON(pr)
		return
	}
	fmt.Fprintf(p.w, "ID:          %s\n", pr.ID)
	fmt.Fprintf(p.w, "Alias:       %s\n", pr.Alias)
	fmt.Fprintf(p.w, "Name:        %s\n", pr.Name)
	if pr.Description != "" {
		fmt.Fprintf(p.w, "Description: %s\n", pr.Description)
	}
	if pr.RepoPath != "" {
		fmt.Fprintf(p.w, "Repo:        %s\n", pr.RepoPath)
	}
	fmt.Fprintf(p.w, "Created:     %s\n", formatTime(pr.CreatedAt))
}

// --- Tasks ---

func (p *Printer) Tasks(tasks []*models.Task, total int) {
	if tasks == nil {
		tasks = []*models.Task{}
	}
	if p.json {
		p.PrintJSON(map[string]interface{}{
			"tasks": tasks,
			"page":  map[string]int{"total": total, "count": len(tasks)},
		})
		return
	}
	if len(tasks) == 0 {
		fmt.Fprintln(p.w, "no tasks found")
		return
	}
	t := newTable(p.w)
	t.AppendHeader(table.Row{"REF", "P", "TYPE", "STATUS", "TITLE", "PROJECT", "LABELS", "ACTOR"})
	for _, task := range tasks {
		proj := ""
		if task.Project != nil {
			proj = task.Project.Alias
		}
		labels := labelNames(task.Labels)
		t.AppendRow(table.Row{
			fmt.Sprintf("TASK-%d", task.Seq),
			colorPriority(task.Priority),
			task.Type,
			colorStatus(string(task.Status)),
			truncate(task.Title, 50),
			proj,
			labels,
			task.Actor.Name,
		})
	}
	t.Render()
	if total > len(tasks) {
		fmt.Fprintf(p.w, "showing %d of %d\n", len(tasks), total)
	}
}

func (p *Printer) Task(task *models.Task) {
	if p.json {
		p.PrintJSON(task)
		return
	}
	proj := ""
	if task.Project != nil {
		proj = task.Project.Alias
	}
	fmt.Fprintf(p.w, "ID:          %s\n", task.ID)
	fmt.Fprintf(p.w, "Ref:         TASK-%d\n", task.Seq)
	fmt.Fprintf(p.w, "Project:     %s\n", proj)
	fmt.Fprintf(p.w, "Title:       %s\n", task.Title)
	fmt.Fprintf(p.w, "Type:        %s\n", task.Type)
	fmt.Fprintf(p.w, "Status:      %s\n", colorStatus(string(task.Status)))
	fmt.Fprintf(p.w, "Priority:    %s\n", colorPriority(task.Priority))
	if task.Assignee != "" {
		fmt.Fprintf(p.w, "Assignee:    %s\n", task.Assignee)
	}
	if task.ProjectPath != "" {
		fmt.Fprintf(p.w, "Path:        %s\n", task.ProjectPath)
	}
	fmt.Fprintf(p.w, "Actor:       %s:%s\n", task.Actor.Kind, task.Actor.Name)
	if task.Source != "" {
		fmt.Fprintf(p.w, "Source:      %s\n", task.Source)
	}
	if task.ExternalRef != "" {
		fmt.Fprintf(p.w, "Ref:         %s\n", task.ExternalRef)
	}
	if len(task.Labels) > 0 {
		fmt.Fprintf(p.w, "Labels:      %s\n", labelNames(task.Labels))
	}
	fmt.Fprintf(p.w, "Created:     %s\n", formatTime(task.CreatedAt))
	if task.Description != "" {
		fmt.Fprintf(p.w, "\n--- Description ---\n%s\n", task.Description)
	}
	if len(task.Plans) > 0 {
		fmt.Fprintf(p.w, "\n--- Plans (%d) ---\n", len(task.Plans))
		for _, pl := range task.Plans {
			v := pl.Version
			if v != nil {
				fmt.Fprintf(p.w, "[v%d] %s (by %s:%s)\n", v.Version, v.Title, v.Actor.Kind, v.Actor.Name)
			}
		}
	}
	if len(task.Comments) > 0 {
		fmt.Fprintf(p.w, "\n--- Comments (%d) ---\n", len(task.Comments))
		for _, c := range task.Comments {
			fmt.Fprintf(p.w, "%s [%s:%s]: %s\n", formatTime(c.CreatedAt), c.Actor.Kind, c.Actor.Name, c.Body)
		}
	}
}

// --- Plans ---

func (p *Printer) Plans(plans []*models.Plan) {
	if p.json {
		p.PrintJSON(map[string]interface{}{"plans": plans})
		return
	}
	if len(plans) == 0 {
		fmt.Fprintln(p.w, "no plans found")
		return
	}
	t := newTable(p.w)
	t.AppendHeader(table.Row{"ID", "VERSION", "TITLE", "ACTOR", "CREATED"})
	for _, pl := range plans {
		v := pl.Version
		if v == nil {
			continue
		}
		t.AppendRow(table.Row{pl.ID[:8], fmt.Sprintf("v%d", v.Version), v.Title,
			fmt.Sprintf("%s:%s", v.Actor.Kind, v.Actor.Name), formatTime(v.CreatedAt)})
	}
	t.Render()
}

func (p *Printer) Plan(pl *models.Plan) {
	if p.json {
		p.PrintJSON(pl)
		return
	}
	v := pl.Version
	if v == nil {
		fmt.Fprintln(p.w, "no version data")
		return
	}
	fmt.Fprintf(p.w, "Plan ID:     %s\n", pl.ID)
	fmt.Fprintf(p.w, "Version:     v%d (current: v%d)\n", v.Version, pl.CurrentVersion)
	fmt.Fprintf(p.w, "Title:       %s\n", v.Title)
	fmt.Fprintf(p.w, "Actor:       %s:%s\n", v.Actor.Kind, v.Actor.Name)
	if v.ChangeNote != "" {
		fmt.Fprintf(p.w, "Note:        %s\n", v.ChangeNote)
	}
	fmt.Fprintf(p.w, "Created:     %s\n", formatTime(v.CreatedAt))
	fmt.Fprintf(p.w, "\n%s\n", v.Body)
}

func (p *Printer) PlanHistory(versions []*models.PlanVersion) {
	if p.json {
		p.PrintJSON(map[string]interface{}{"versions": versions})
		return
	}
	t := newTable(p.w)
	t.AppendHeader(table.Row{"VER", "TITLE", "ACTOR", "NOTE", "CREATED"})
	for _, v := range versions {
		t.AppendRow(table.Row{fmt.Sprintf("v%d", v.Version), v.Title,
			fmt.Sprintf("%s:%s", v.Actor.Kind, v.Actor.Name), v.ChangeNote, formatTime(v.CreatedAt)})
	}
	t.Render()
}

// --- Comments ---

func (p *Printer) Comments(comments []*models.Comment) {
	if p.json {
		p.PrintJSON(map[string]interface{}{"comments": comments})
		return
	}
	for _, c := range comments {
		fmt.Fprintf(p.w, "[%s] %s:%s\n  %s\n\n", formatTime(c.CreatedAt), c.Actor.Kind, c.Actor.Name, c.Body)
	}
}

// --- Labels ---

func (p *Printer) Labels(labels []*models.Label) {
	if p.json {
		p.PrintJSON(map[string]interface{}{"labels": labels})
		return
	}
	t := newTable(p.w)
	t.AppendHeader(table.Row{"NAME", "COLOR"})
	for _, l := range labels {
		t.AppendRow(table.Row{l.Name, l.Color})
	}
	t.Render()
}

// --- helpers ---

func newTable(w io.Writer) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = true
	t.Style().Options.SeparateHeader = true
	t.Style().Format.Header = text.FormatUpper
	return t
}

func formatTime(ns int64) string {
	return time.Unix(0, ns).Local().Format("2006-01-02 15:04")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func labelNames(labels []models.Label) string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return strings.Join(names, ", ")
}

func colorPriority(p int) string {
	switch p {
	case 1:
		return color.RedString("P1")
	case 2:
		return color.YellowString("P2")
	case 3:
		return fmt.Sprintf("P%d", p)
	case 4:
		return color.CyanString("P4")
	case 5:
		return color.HiBlackString("P5")
	}
	return fmt.Sprintf("P%d", p)
}

func colorStatus(s string) string {
	switch s {
	case "todo":
		return s
	case "doing":
		return color.BlueString(s)
	case "done":
		return color.GreenString(s)
	}
	return s
}
