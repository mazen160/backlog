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

type MemoryService struct {
	memory   *repo.MemoryRepo
	projects *repo.ProjectRepo
	activity *repo.ActivityRepo
}

func NewMemoryService(db *sql.DB) *MemoryService {
	return &MemoryService{
		memory:   repo.NewMemoryRepo(db),
		projects: repo.NewProjectRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *MemoryService) Add(ctx context.Context, in models.CreateMemoryInput) (*models.Memory, error) {
	if in.Body == "" {
		return nil, fmt.Errorf("body is required")
	}
	m := &models.Memory{
		ID:        ids.New(),
		ProjectID: in.ProjectID,
		Body:      in.Body,
		Tags:      in.Tags,
		Actor:     in.Actor,
		CreatedAt: timeutil.Now(),
	}
	if err := s.memory.Insert(ctx, m); err != nil {
		return nil, fmt.Errorf("add memory: %w", err)
	}
	s.logActivity(ctx, in.ProjectID, "memory", m.ID, "created",
		"Created memory entry", in.Actor)
	return m, nil
}

func (s *MemoryService) Append(ctx context.Context, id, input string, actor models.Actor) (*models.Memory, error) {
	m, err := s.memory.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("memory entry not found: %w", err)
	}
	if input == "" {
		return nil, fmt.Errorf("body is required")
	}
	joined := joinWithNewline(m.Body, input)
	if err := s.memory.UpdateBody(ctx, id, joined); err != nil {
		return nil, fmt.Errorf("append memory: %w", err)
	}
	m.Body = joined
	s.logActivity(ctx, m.ProjectID, "memory", id, "appended",
		"Appended to memory entry", actor)
	return m, nil
}

func (s *MemoryService) List(ctx context.Context, projectAlias, tag string) ([]*models.Memory, error) {
	if projectAlias == "" {
		return s.memory.ListAcrossProjects(ctx, tag)
	}
	p, err := s.projects.GetByAlias(ctx, projectAlias)
	if err != nil {
		return nil, fmt.Errorf("project %q: %w", projectAlias, err)
	}
	return s.memory.ListForProject(ctx, p.ID, tag)
}

func (s *MemoryService) Update(ctx context.Context, id, body, tags string) (*models.Memory, error) {
	m, err := s.memory.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("memory entry not found: %w", err)
	}
	if body == "" {
		return nil, fmt.Errorf("body is required")
	}
	if err := s.memory.UpdateBodyAndTags(ctx, id, body, tags); err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}
	m.Body = body
	m.Tags = tags
	return m, nil
}

func (s *MemoryService) Delete(ctx context.Context, id string) error {
	m, err := s.memory.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("memory entry not found: %w", err)
	}
	s.logActivity(ctx, m.ProjectID, "memory", id, "deleted",
		"Deleted memory entry", m.Actor)
	return s.memory.Delete(ctx, id)
}

func (s *MemoryService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
