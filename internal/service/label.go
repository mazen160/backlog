package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
)

type LabelService struct {
	labels   *repo.LabelRepo
	projects *repo.ProjectRepo
	activity *repo.ActivityRepo
}

func NewLabelService(db *sql.DB) *LabelService {
	return &LabelService{
		labels:   repo.NewLabelRepo(db),
		projects: repo.NewProjectRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *LabelService) Create(ctx context.Context, in models.CreateLabelInput) (*models.Label, error) {
	if in.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	// Upsert behaviour: return existing if name already exists
	existing, err := s.labels.GetByName(ctx, in.ProjectID, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	l := &models.Label{
		ID:        ids.New(),
		ProjectID: in.ProjectID,
		Name:      in.Name,
		Color:     in.Color,
	}
	if err := s.labels.Insert(ctx, l); err != nil {
		return nil, fmt.Errorf("create label: %w", err)
	}
	s.logActivity(ctx, in.ProjectID, "label", l.ID, "created",
		fmt.Sprintf("Created label %q", in.Name), in.Actor)
	return l, nil
}

func (s *LabelService) ListForProject(ctx context.Context, projectID string) ([]*models.Label, error) {
	return s.labels.ListForProject(ctx, projectID)
}

func (s *LabelService) AttachByName(ctx context.Context, projectID, taskID, labelName string, actor ...models.Actor) error {
	act := firstActor(actor)
	l, err := s.labels.GetByName(ctx, projectID, labelName)
	if err != nil {
		return err
	}
	if l == nil {
		// Auto-create label
		l, err = s.Create(ctx, models.CreateLabelInput{
			ProjectID: projectID,
			Name:      labelName,
			Actor:     act,
		})
		if err != nil {
			return err
		}
	}
	if err := s.labels.Attach(ctx, taskID, l.ID); err != nil {
		return err
	}
	s.logActivity(ctx, projectID, "label", l.ID, "attached",
		fmt.Sprintf("Attached label %q to task %s", labelName, taskID[:8]), act)
	return nil
}

func (s *LabelService) Attach(ctx context.Context, taskID, labelID string) error {
	return s.labels.Attach(ctx, taskID, labelID)
}

func (s *LabelService) Detach(ctx context.Context, taskID string, projectID string, labelName string, actor ...models.Actor) error {
	act := firstActor(actor)
	l, err := s.labels.GetByName(ctx, projectID, labelName)
	if err != nil {
		return err
	}
	if l == nil {
		return fmt.Errorf("label %q not found", labelName)
	}
	if err := s.labels.Detach(ctx, taskID, l.ID); err != nil {
		return err
	}
	s.logActivity(ctx, projectID, "label", l.ID, "detached",
		fmt.Sprintf("Detached label %q from task %s", labelName, taskID[:8]), act)
	return nil
}

func (s *LabelService) Delete(ctx context.Context, projectID, name string, actor ...models.Actor) error {
	act := firstActor(actor)
	l, err := s.labels.GetByName(ctx, projectID, name)
	if err != nil {
		return err
	}
	if l == nil {
		return fmt.Errorf("label %q not found", name)
	}
	s.logActivity(ctx, projectID, "label", l.ID, "deleted",
		fmt.Sprintf("Deleted label %q", name), act)
	return s.labels.Delete(ctx, l.ID)
}

func firstActor(actors []models.Actor) models.Actor {
	if len(actors) == 0 {
		return models.Actor{}
	}
	return actors[0]
}

func (s *LabelService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
