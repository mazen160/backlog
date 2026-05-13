package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/timeutil"
)

var aliasRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type ProjectService struct {
	db       *sql.DB
	projects *repo.ProjectRepo
	activity *repo.ActivityRepo
}

func NewProjectService(db *sql.DB) *ProjectService {
	return &ProjectService{
		db:       db,
		projects: repo.NewProjectRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *ProjectService) Create(ctx context.Context, in models.CreateProjectInput) (*models.Project, error) {
	if in.Alias == "" {
		return nil, fmt.Errorf("alias is required")
	}
	if !aliasRe.MatchString(in.Alias) {
		return nil, fmt.Errorf("alias must be lowercase alphanumeric with optional hyphens")
	}
	if len(in.Alias) > 64 {
		return nil, fmt.Errorf("alias exceeds max length of 64 characters")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(in.Name) > 255 {
		return nil, fmt.Errorf("name exceeds max length of 255 characters")
	}
	now := timeutil.Now()
	p := &models.Project{
		ID:          ids.New(),
		Alias:       in.Alias,
		Name:        in.Name,
		Description: in.Description,
		RepoPath:    in.RepoPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.projects.Insert(ctx, p); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	s.log(ctx, p.ID, "project", p.ID, "created",
		fmt.Sprintf("Created project %q (%s)", p.Name, p.Alias), in.Actor)
	return p, nil
}

func (s *ProjectService) List(ctx context.Context, includeArchived bool) ([]*models.Project, error) {
	return s.projects.List(ctx, includeArchived)
}

func (s *ProjectService) GetByAlias(ctx context.Context, alias string) (*models.Project, error) {
	p, err := s.projects.GetByAlias(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("project %q: %w", alias, err)
	}
	return p, nil
}

func (s *ProjectService) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return s.projects.GetByID(ctx, id)
}

func (s *ProjectService) Update(ctx context.Context, alias string, in models.UpdateProjectInput, actor models.Actor) (*models.Project, error) {
	p, err := s.projects.GetByAlias(ctx, alias)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.RepoPath != nil {
		p.RepoPath = *in.RepoPath
	}
	p.UpdatedAt = timeutil.Now()
	if err := s.projects.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	s.log(ctx, p.ID, "project", p.ID, "updated",
		fmt.Sprintf("Updated project %q (%s)", p.Name, p.Alias), actor)
	return p, nil
}

func (s *ProjectService) Archive(ctx context.Context, alias string, actor models.Actor) (*models.Project, error) {
	p, err := s.projects.GetByAlias(ctx, alias)
	if err != nil {
		return nil, err
	}
	now := timeutil.Now()
	p.ArchivedAt = &now
	p.UpdatedAt = now
	if err := s.projects.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("archive project: %w", err)
	}
	s.log(ctx, p.ID, "project", p.ID, "archived",
		fmt.Sprintf("Archived project %q (%s)", p.Name, p.Alias), actor)
	return p, nil
}

func (s *ProjectService) Unarchive(ctx context.Context, alias string, actor models.Actor) (*models.Project, error) {
	p, err := s.projects.GetByAlias(ctx, alias)
	if err != nil {
		return nil, err
	}
	if p.ArchivedAt == nil {
		return p, nil
	}
	p.ArchivedAt = nil
	p.UpdatedAt = timeutil.Now()
	if err := s.projects.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("unarchive project: %w", err)
	}
	s.log(ctx, p.ID, "project", p.ID, "unarchived",
		fmt.Sprintf("Restored project %q (%s)", p.Name, p.Alias), actor)
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, alias string, actor models.Actor) error {
	p, err := s.projects.GetByAlias(ctx, alias)
	if err != nil {
		return err
	}
	if p.ArchivedAt == nil {
		return fmt.Errorf("project %q must be archived before it can be deleted", alias)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Remove attachments linked to this project's tasks and docs.
	// These have no FK to projects so cascades don't reach them.
	if _, err = tx.ExecContext(ctx, `
		DELETE FROM attachments
		WHERE (linked_type='task' AND linked_id IN (SELECT id FROM tasks       WHERE project_id=?))
		   OR (linked_type='doc'  AND linked_id IN (SELECT id FROM project_docs WHERE project_id=?))`,
		p.ID, p.ID); err != nil {
		return fmt.Errorf("delete attachments: %w", err)
	}

	// Activity rows have no FK — delete them so no orphaned history remains.
	if _, err = tx.ExecContext(ctx, `DELETE FROM activity_log WHERE project_id=?`, p.ID); err != nil {
		return fmt.Errorf("delete activity log: %w", err)
	}

	// Delete the project; FK cascades clean up tasks, docs, labels, plans, comments.
	if _, err = tx.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, p.ID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	return tx.Commit()
}

func (s *ProjectService) log(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) error {
	a := &models.Activity{
		ID:        ids.New(),
		ProjectID: projectID,
		Entity:    entity,
		EntityID:  entityID,
		Action:    action,
		Summary:   summary,
		Actor:     actor,
		Payload:   "{}",
		CreatedAt: timeutil.Now(),
	}
	return s.activity.Insert(ctx, a)
}
