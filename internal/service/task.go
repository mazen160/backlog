package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/timeutil"
)

type TaskService struct {
	tasks    *repo.TaskRepo
	projects *repo.ProjectRepo
	labels   *repo.LabelRepo
	plans    *repo.PlanRepo
	comments *repo.CommentRepo
	activity *repo.ActivityRepo
	plan     *PlanService
	label    *LabelService
}

func NewTaskService(db *sql.DB, ps *PlanService, ls *LabelService) *TaskService {
	return &TaskService{
		tasks:    repo.NewTaskRepo(db),
		projects: repo.NewProjectRepo(db),
		labels:   repo.NewLabelRepo(db),
		plans:    repo.NewPlanRepo(db),
		comments: repo.NewCommentRepo(db),
		activity: repo.NewActivityRepo(db),
		plan:     ps,
		label:    ls,
	}
}

const (
	maxTitleLen       = 255
	maxDescriptionLen = 65535
)

func joinTaskTypes() string {
	parts := make([]string, 0, len(models.AllTaskTypes()))
	for _, t := range models.AllTaskTypes() {
		parts = append(parts, string(t))
	}
	return strings.Join(parts, ", ")
}

func joinTaskStatuses() string {
	parts := make([]string, 0, len(models.AllTaskStatuses()))
	for _, s := range models.AllTaskStatuses() {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}

func (s *TaskService) Create(ctx context.Context, in models.CreateTaskInput) (*models.Task, error) {
	if in.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(in.Title) > maxTitleLen {
		return nil, fmt.Errorf("title exceeds max length of %d characters", maxTitleLen)
	}
	if len(in.Description) > maxDescriptionLen {
		return nil, fmt.Errorf("description exceeds max length of %d characters", maxDescriptionLen)
	}
	if in.ProjectID == "" {
		return nil, fmt.Errorf("project is required")
	}
	if in.Type == "" {
		in.Type = models.TaskTypeTask
	}
	if !in.Type.Valid() {
		return nil, fmt.Errorf("invalid type %q (valid: %s)", in.Type, joinTaskTypes())
	}
	if in.Status == "" {
		in.Status = models.TaskStatusTodo
	}
	if !in.Status.Valid() {
		return nil, fmt.Errorf("invalid status %q (valid: %s)", in.Status, joinTaskStatuses())
	}
	if in.Priority == 0 {
		in.Priority = 3
	}
	if in.Priority < 1 || in.Priority > 5 {
		return nil, fmt.Errorf("priority must be 1-5")
	}

	now := timeutil.Now()
	t := &models.Task{
		ID:          ids.New(),
		ProjectID:   in.ProjectID,
		Title:       in.Title,
		Description: in.Description,
		Type:        in.Type,
		Status:      in.Status,
		Priority:    in.Priority,
		Assignee:    in.Assignee,
		DueAt:       in.DueAt,
		Actor:       in.Actor,
		Source:      in.Source,
		ExternalRef: in.ExternalRef,
		ProjectPath: in.ProjectPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.tasks.Insert(ctx, t); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	// If label or plan attachment fails partway through, the task and any
	// already-attached labels/plans must be rolled back so the caller never
	// sees a half-populated task. FK ON DELETE CASCADE on task_labels and
	// plans means deleting the task wipes any partial children.
	rollback := func(cause error, format string, args ...any) error {
		if delErr := s.tasks.Delete(ctx, t.ID); delErr != nil {
			return fmt.Errorf(format+" (rollback also failed: %v)", append(args, delErr)...)
		}
		return fmt.Errorf(format, args...)
	}

	for _, labelName := range in.Labels {
		if err := s.label.AttachByName(ctx, t.ProjectID, t.ID, labelName, in.Actor); err != nil {
			return nil, rollback(err, "attach label %q: %w", labelName, err)
		}
	}

	for _, pi := range in.Plans {
		pi.TaskID = t.ID
		pi.Actor = in.Actor
		if _, err := s.plan.Create(ctx, pi); err != nil {
			return nil, rollback(err, "create plan: %w", err)
		}
	}

	s.logActivity(ctx, in.ProjectID, "task", t.ID, "created",
		fmt.Sprintf("Created task %q (TASK-%d)", t.Title, t.Seq), in.Actor)
	return t, nil
}

// ResolveRef is the public form of resolveID for use by other CLI commands.
func (s *TaskService) ResolveRef(ctx context.Context, ref string) (string, error) {
	return s.resolveID(ctx, ref)
}

// resolveID accepts a ULID, a bare integer ("42"), or a TASK-N reference ("TASK-42").
func (s *TaskService) resolveID(ctx context.Context, ref string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(ref))
	numStr := upper
	if strings.HasPrefix(upper, "TASK-") {
		numStr = upper[5:]
	}
	if n, err := strconv.Atoi(numStr); err == nil {
		t, err := s.tasks.GetBySeq(ctx, n)
		if err != nil {
			return "", fmt.Errorf("task TASK-%d not found", n)
		}
		return t.ID, nil
	}
	return ref, nil
}

func (s *TaskService) Get(ctx context.Context, ref string, withPlans, withComments bool) (*models.Task, error) {
	id, err := s.resolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := s.projects.GetByID(ctx, t.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("load project for task %s: %w", t.ID, err)
	}
	t.Project = p
	labels, err := s.labelsForTask(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("load labels for task %s: %w", t.ID, err)
	}
	t.Labels = labels
	if withPlans {
		plans, err := s.plans.ListPlansForTask(ctx, t.ID, false)
		if err != nil {
			return nil, fmt.Errorf("load plans for task %s: %w", t.ID, err)
		}
		for _, pl := range plans {
			t.Plans = append(t.Plans, *pl)
		}
	}
	if withComments {
		comments, err := s.comments.ListForTask(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("load comments for task %s: %w", t.ID, err)
		}
		for _, c := range comments {
			t.Comments = append(t.Comments, *c)
		}
	}
	return t, nil
}

func (s *TaskService) List(ctx context.Context, f models.TaskFilter) ([]*models.Task, int, error) {
	var tasks []*models.Task
	var err error
	if f.Search != "" {
		tasks, err = s.tasks.ListBySearch(ctx, f)
	} else {
		tasks, err = s.tasks.List(ctx, f)
	}
	if err != nil {
		return nil, 0, err
	}
	// Attach labels
	for _, t := range tasks {
		t.Labels, _ = s.labelsForTask(ctx, t.ID)
	}
	// Always run a separate count when paginating so the total reflects the
	// full result set, not just the current page.
	total := len(tasks)
	if f.Limit > 0 {
		fCount := f
		fCount.Limit = 0
		fCount.Offset = 0
		if n, err := s.tasks.Count(ctx, fCount); err == nil {
			total = n
		}
	}
	return tasks, total, nil
}

func (s *TaskService) Update(ctx context.Context, ref string, in models.UpdateTaskInput, actor models.Actor) (*models.Task, error) {
	id, err := s.resolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	statusChanged := in.Status != nil && *in.Status != t.Status
	oldStatus := t.Status

	if in.Title != nil {
		if len(*in.Title) > maxTitleLen {
			return nil, fmt.Errorf("title exceeds max length of %d characters", maxTitleLen)
		}
		t.Title = *in.Title
	}
	if in.Description != nil {
		if len(*in.Description) > maxDescriptionLen {
			return nil, fmt.Errorf("description exceeds max length of %d characters", maxDescriptionLen)
		}
		t.Description = *in.Description
	}
	if in.Type != nil {
		if !in.Type.Valid() {
			return nil, fmt.Errorf("invalid type %q (valid: %s)", *in.Type, joinTaskTypes())
		}
		t.Type = *in.Type
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, fmt.Errorf("invalid status %q (valid: %s)", *in.Status, joinTaskStatuses())
		}
		if *in.Status == models.TaskStatusDone && t.Status != models.TaskStatusDone {
			now := timeutil.Now()
			t.CompletedAt = &now
		}
		t.Status = *in.Status
	}
	if in.Priority != nil {
		if *in.Priority < 1 || *in.Priority > 5 {
			return nil, fmt.Errorf("priority must be 1-5")
		}
		t.Priority = *in.Priority
	}
	if in.Assignee != nil {
		t.Assignee = *in.Assignee
	}
	if in.ClearDueAt {
		t.DueAt = nil
	} else if in.DueAt != nil {
		t.DueAt = in.DueAt
	}
	if in.Source != nil {
		t.Source = *in.Source
	}
	if in.ExternalRef != nil {
		t.ExternalRef = *in.ExternalRef
	}
	if in.ProjectPath != nil {
		t.ProjectPath = *in.ProjectPath
	}
	t.UpdatedAt = timeutil.Now()
	if err := s.tasks.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	if statusChanged {
		s.logActivity(ctx, t.ProjectID, "task", t.ID, "status_changed",
			fmt.Sprintf("Changed status of TASK-%d %q from %s to %s", t.Seq, t.Title, oldStatus, t.Status), actor)
	} else {
		s.logActivity(ctx, t.ProjectID, "task", t.ID, "updated",
			fmt.Sprintf("Updated task TASK-%d %q", t.Seq, t.Title), actor)
	}
	return t, nil
}

func (s *TaskService) Move(ctx context.Context, id string, status models.TaskStatus, actor models.Actor) (*models.Task, error) {
	st := status
	return s.Update(ctx, id, models.UpdateTaskInput{Status: &st}, actor)
}

func (s *TaskService) Archive(ctx context.Context, ref string, actor models.Actor) (*models.Task, error) {
	id, err := s.resolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := timeutil.Now()
	t.ArchivedAt = &now
	t.UpdatedAt = now
	if err := s.tasks.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("archive task: %w", err)
	}
	s.logActivity(ctx, t.ProjectID, "task", t.ID, "archived",
		fmt.Sprintf("Archived task TASK-%d %q", t.Seq, t.Title), actor)
	return t, nil
}

func (s *TaskService) Unarchive(ctx context.Context, ref string, actor models.Actor) (*models.Task, error) {
	id, err := s.resolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.ArchivedAt == nil {
		return t, nil
	}
	t.ArchivedAt = nil
	t.UpdatedAt = timeutil.Now()
	if err := s.tasks.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("unarchive task: %w", err)
	}
	s.logActivity(ctx, t.ProjectID, "task", t.ID, "unarchived",
		fmt.Sprintf("Restored task TASK-%d %q", t.Seq, t.Title), actor)
	return t, nil
}

func (s *TaskService) Delete(ctx context.Context, ref string, actor models.Actor) error {
	id, err := s.resolveID(ctx, ref)
	if err != nil {
		return err
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.tasks.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	s.logActivity(ctx, t.ProjectID, "task", t.ID, "deleted",
		fmt.Sprintf("Deleted task TASK-%d %q", t.Seq, t.Title), actor)
	return nil
}

func (s *TaskService) labelsForTask(ctx context.Context, taskID string) ([]models.Label, error) {
	ls, err := s.labels.ListForTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	result := make([]models.Label, len(ls))
	for i, l := range ls {
		result[i] = *l
	}
	return result, nil
}

func (s *TaskService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
