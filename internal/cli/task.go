package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Manage tasks", Aliases: []string{"t"}}
	cmd.AddCommand(
		taskAddCmd(),
		taskListCmd(),
		taskShowCmd(),
		taskUpdateCmd(),
		taskMoveCmd(),
		taskArchiveCmd(),
		taskDeleteCmd(),
	)
	return cmd
}

// newTaskSvc builds the standard TaskService wiring used by every task
// subcommand. Kept as a helper because the three-line construction was
// repeated verbatim across seven RunE bodies.
func newTaskSvc() *service.TaskService {
	planSvc := service.NewPlanService(app.DB)
	labelSvc := service.NewLabelService(app.DB)
	return service.NewTaskService(app.DB, planSvc, labelSvc)
}

func taskAddCmd() *cobra.Command {
	var (
		project     string
		title       string
		description string
		taskType    string
		status      string
		priority    string
		assignee    string
		labels      []string
		source      string
		externalRef string
		projectPath string
		fromFile    string
		dueDate     string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			projSvc := service.NewProjectService(app.DB)
			projectAlias := projectOrDefault(cmd, project)

			var in models.CreateTaskInput

			// Load from file if provided
			if fromFile != "" {
				ext := strings.ToLower(filepath.Ext(fromFile))
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				if ext == ".json" {
					// Full task payload
					if err := json.Unmarshal(data, &in); err != nil {
						return fmt.Errorf("parse JSON: %w", err)
					}
					// Resolve project alias to ID
					if in.ProjectID == "" && projectAlias != "" {
						p, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
						if err != nil {
							return err
						}
						in.ProjectID = p.ID
					} else if in.ProjectID != "" && !ids.IsULID(in.ProjectID) {
						// Not a ULID, so treat as a project alias.
						p, err := projSvc.GetByAlias(cmd.Context(), in.ProjectID)
						if err != nil {
							return err
						}
						in.ProjectID = p.ID
					}
				} else {
					// Treat as description file
					description = string(data)
				}
			}

			// Overlay CLI flags
			if cmd.Flags().Changed("project") {
				p, err := projSvc.GetByAlias(cmd.Context(), project)
				if err != nil {
					return err
				}
				in.ProjectID = p.ID
			} else if in.ProjectID == "" && projectAlias != "" {
				p, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
				if err != nil {
					return err
				}
				in.ProjectID = p.ID
			}
			if title != "" {
				in.Title = title
			}
			if description != "" {
				in.Description = description
			}
			if taskType != "" {
				in.Type = models.TaskType(taskType)
			}
			if status != "" {
				in.Status = models.TaskStatus(status)
			}
			if priority != "" {
				p, err := parsePriorityFlag(priority)
				if err != nil {
					return err
				}
				in.Priority = p
			}
			if assignee != "" {
				in.Assignee = assignee
			}
			if len(labels) > 0 {
				in.Labels = labels
			}
			if source != "" {
				in.Source = source
			}
			if externalRef != "" {
				in.ExternalRef = externalRef
			}
			if projectPath != "" {
				in.ProjectPath = projectPath
			}
			if dueDate != "" {
				ts, err := parseDateFlag(dueDate)
				if err != nil {
					return fmt.Errorf("--due-date: %w", err)
				}
				in.DueAt = &ts
			}
			in.Actor = app.Actor

			t, err := svc.Create(cmd.Context(), in)
			if err != nil {
				return err
			}
			app.Out.Task(t)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project alias")
	cmd.Flags().StringVarP(&title, "title", "t", "", "task title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "task description")
	cmd.Flags().StringVar(&taskType, "type", "", "task type (task|bug|issue|improvement|feature|vulnerability|chore|spike|bucket-list)")
	cmd.Flags().StringVar(&status, "status", "", "status (todo|doing|done)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (P1-P5 or 1-5)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "assignee")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "labels (can repeat)")
	cmd.Flags().StringVar(&source, "source", "", "source (e.g. security-review)")
	cmd.Flags().StringVar(&externalRef, "external-ref", "", "external reference URL or ID")
	cmd.Flags().StringVar(&projectPath, "project-path", "", "file path or URL for relevant code location (e.g. internal/handlers/search.go:84)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read task from a .json or .md file")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "due date (YYYY-MM-DD or RFC3339)")
	return cmd
}

func taskListCmd() *cobra.Command {
	var (
		project         string
		status          string
		taskType        string
		priority        string
		assignee        string
		labels          []string
		actorKind       string
		actorName       string
		source          string
		search          string
		includeArchived bool
		limit           int
		offset          int
		sort            string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()

			f := models.TaskFilter{
				ProjectAlias:    projectOrDefault(cmd, project),
				Status:          models.TaskStatus(status),
				Type:            models.TaskType(taskType),
				Assignee:        assignee,
				Labels:          labels,
				ActorKind:       models.ActorKind(actorKind),
				ActorName:       actorName,
				Source:          source,
				Search:          search,
				IncludeArchived: includeArchived,
				Limit:           limit,
				Offset:          offset,
				Sort:            sort,
			}
			if priority != "" {
				p, err := parsePriorityFlag(priority)
				if err != nil {
					return err
				}
				f.Priority = p
			}

			tasks, total, err := svc.List(cmd.Context(), f)
			if err != nil {
				return err
			}
			app.Out.Tasks(tasks, total)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project alias")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&taskType, "type", "", "filter by type")
	cmd.Flags().StringVar(&priority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "filter by labels")
	cmd.Flags().StringVar(&actorKind, "actor-kind", "", "filter by actor kind (human|ai)")
	cmd.Flags().StringVar(&actorName, "actor-name", "", "filter by actor name")
	cmd.Flags().StringVar(&source, "source", "", "filter by source")
	cmd.Flags().StringVar(&search, "search", "", "full-text search")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "include archived tasks")
	cmd.Flags().IntVar(&limit, "limit", 50, "max results")
	cmd.Flags().IntVar(&offset, "offset", 0, "offset for pagination")
	cmd.Flags().StringVar(&sort, "sort", "", "sort order: priority (default), created, updated, seq, title")
	return cmd
}

func taskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			t, err := svc.Get(cmd.Context(), args[0], true, true)
			if err != nil {
				return err
			}
			app.Out.Task(t)
			return nil
		},
	}
}

func taskUpdateCmd() *cobra.Command {
	var (
		title       string
		description string
		taskType    string
		status      string
		priority    string
		assignee    string
		source      string
		externalRef string
		projectPath string
		dueDate     string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			in := models.UpdateTaskInput{}
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if cmd.Flags().Changed("description") {
				in.Description = &description
			}
			if cmd.Flags().Changed("type") {
				tt := models.TaskType(taskType)
				in.Type = &tt
			}
			if cmd.Flags().Changed("status") {
				st := models.TaskStatus(status)
				in.Status = &st
			}
			if cmd.Flags().Changed("priority") {
				p, err := parsePriorityFlag(priority)
				if err != nil {
					return err
				}
				in.Priority = &p
			}
			if cmd.Flags().Changed("assignee") {
				in.Assignee = &assignee
			}
			if cmd.Flags().Changed("source") {
				in.Source = &source
			}
			if cmd.Flags().Changed("external-ref") {
				in.ExternalRef = &externalRef
			}
			if cmd.Flags().Changed("project-path") {
				in.ProjectPath = &projectPath
			}
			if cmd.Flags().Changed("due-date") {
				ts, err := parseDateFlag(dueDate)
				if err != nil {
					return fmt.Errorf("--due-date: %w", err)
				}
				in.DueAt = &ts
			}
			t, err := svc.Update(cmd.Context(), args[0], in, app.Actor)
			if err != nil {
				return err
			}
			app.Out.Task(t)
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "new title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "new description")
	cmd.Flags().StringVar(&taskType, "type", "", "new type")
	cmd.Flags().StringVar(&status, "status", "", "new status")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority")
	cmd.Flags().StringVar(&assignee, "assignee", "", "new assignee")
	cmd.Flags().StringVar(&source, "source", "", "new source")
	cmd.Flags().StringVar(&externalRef, "external-ref", "", "new external ref")
	cmd.Flags().StringVar(&projectPath, "project-path", "", "file path or URL for relevant code location")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "due date (YYYY-MM-DD or RFC3339)")
	return cmd
}

func taskMoveCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Move task to a status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			t, err := svc.Move(cmd.Context(), args[0], models.TaskStatus(status), app.Actor)
			if err != nil {
				return err
			}
			app.Out.Task(t)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "target status (todo|doing|done)")
	cmd.MarkFlagRequired("status")
	return cmd
}

func taskArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			t, err := svc.Archive(cmd.Context(), args[0], app.Actor)
			if err != nil {
				return err
			}
			ref := fmt.Sprintf("TASK-%d", t.Seq)
			app.Out.CommandResult("archive", "task", t.ID, ref, "task archived: "+ref)
			return nil
		},
	}
}

func taskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newTaskSvc()
			t, err := svc.Get(cmd.Context(), args[0], false, false)
			if err != nil {
				return err
			}
			if err := svc.Delete(cmd.Context(), args[0], app.Actor); err != nil {
				return err
			}
			ref := fmt.Sprintf("TASK-%d", t.Seq)
			app.Out.CommandResult("delete", "task", t.ID, ref, "task deleted: "+ref)
			return nil
		},
	}
}

func parsePriorityFlag(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "P")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 5 {
		return 0, fmt.Errorf("invalid priority %q (use P1-P5 or 1-5)", s)
	}
	return n, nil
}

func parseDateFlag(s string) (int64, error) {
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UnixNano(), nil
		}
	}
	return 0, fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", s)
}
