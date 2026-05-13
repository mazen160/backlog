package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/timeutil"
)

type PlanService struct {
	plans    *repo.PlanRepo
	tasks    *repo.TaskRepo
	activity *repo.ActivityRepo
}

func NewPlanService(db *sql.DB) *PlanService {
	return &PlanService{
		plans:    repo.NewPlanRepo(db),
		tasks:    repo.NewTaskRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *PlanService) Create(ctx context.Context, in models.CreatePlanInput) (*models.Plan, error) {
	if in.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if in.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if in.Body == "" {
		return nil, fmt.Errorf("body is required")
	}
	now := timeutil.Now()
	planID := ids.New()
	p := &models.Plan{
		ID:             planID,
		TaskID:         in.TaskID,
		CurrentVersion: 1,
		Source:         in.Source,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.plans.InsertPlan(ctx, p); err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	v := &models.PlanVersion{
		ID:         ids.New(),
		PlanID:     planID,
		Version:    1,
		Title:      in.Title,
		Body:       in.Body,
		Actor:      in.Actor,
		ChangeNote: in.ChangeNote,
		CreatedAt:  now,
	}
	if err := s.plans.InsertVersion(ctx, v); err != nil {
		return nil, fmt.Errorf("create plan version: %w", err)
	}
	p.Version = v

	projectID, err := s.taskProjectID(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, projectID, "plan", planID, "created",
		fmt.Sprintf("Created plan %q on task %s", in.Title, in.TaskID[:8]), in.Actor)
	return p, nil
}

func (s *PlanService) Update(ctx context.Context, planID string, in models.UpdatePlanInput) (*models.Plan, error) {
	p, err := s.plans.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	if in.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if in.Body == "" {
		return nil, fmt.Errorf("body is required")
	}
	newVersion := p.CurrentVersion + 1
	now := timeutil.Now()
	v := &models.PlanVersion{
		ID:         ids.New(),
		PlanID:     planID,
		Version:    newVersion,
		Title:      in.Title,
		Body:       in.Body,
		Actor:      in.Actor,
		ChangeNote: in.ChangeNote,
		CreatedAt:  now,
	}
	if err := s.plans.InsertVersion(ctx, v); err != nil {
		return nil, fmt.Errorf("create plan version: %w", err)
	}
	if err := s.plans.BumpVersion(ctx, planID, newVersion, now); err != nil {
		return nil, fmt.Errorf("bump plan version: %w", err)
	}
	p.CurrentVersion = newVersion
	p.UpdatedAt = now
	p.Version = v

	projectID, err := s.taskProjectID(ctx, p.TaskID)
	if err != nil {
		return nil, err
	}
	s.logActivity(ctx, projectID, "plan", planID, "version_added",
		fmt.Sprintf("Added plan version v%d %q", newVersion, in.Title), in.Actor)
	return p, nil
}

func (s *PlanService) Get(ctx context.Context, planID string, version int) (*models.Plan, error) {
	p, err := s.plans.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		version = p.CurrentVersion
	}
	v, err := s.plans.GetVersion(ctx, planID, version)
	if err != nil {
		return nil, err
	}
	p.Version = v
	return p, nil
}

func (s *PlanService) ListForTask(ctx context.Context, taskID string) ([]*models.Plan, error) {
	return s.plans.ListPlansForTask(ctx, taskID, false)
}

func (s *PlanService) ListForProject(ctx context.Context, projectAlias string) ([]*repo.PlanWithTask, error) {
	return s.plans.ListPlansForProject(ctx, projectAlias)
}

func (s *PlanService) History(ctx context.Context, planID string) ([]*models.PlanVersion, error) {
	if _, err := s.plans.GetPlan(ctx, planID); err != nil {
		return nil, err
	}
	return s.plans.ListVersionHistory(ctx, planID)
}

func (s *PlanService) Archive(ctx context.Context, planID string, actor models.Actor) error {
	p, err := s.plans.GetPlan(ctx, planID)
	if err != nil {
		return err
	}
	if err := s.plans.ArchivePlan(ctx, planID, timeutil.Now()); err != nil {
		return fmt.Errorf("archive plan: %w", err)
	}
	projectID, err := s.taskProjectID(ctx, p.TaskID)
	if err != nil {
		return err
	}
	s.logActivity(ctx, projectID, "plan", planID, "archived",
		fmt.Sprintf("Archived plan %s", planID[:8]), actor)
	return nil
}

func (s *PlanService) Delete(ctx context.Context, planID string, actor models.Actor) error {
	p, err := s.plans.GetPlan(ctx, planID)
	if err != nil {
		return err
	}
	projectID, err := s.taskProjectID(ctx, p.TaskID)
	if err != nil {
		return err
	}
	if err := s.plans.DeletePlan(ctx, planID); err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	s.logActivity(ctx, projectID, "plan", planID, "deleted",
		fmt.Sprintf("Deleted plan %s", planID[:8]), actor)
	return nil
}

func (s *PlanService) taskProjectID(ctx context.Context, taskID string) (string, error) {
	t, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("resolve project for task %s: %w", taskID, err)
	}
	return t.ProjectID, nil
}

func (s *PlanService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
