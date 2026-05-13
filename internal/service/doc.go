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

type DocService struct {
	docs     *repo.DocRepo
	projects *repo.ProjectRepo
	activity *repo.ActivityRepo
}

func NewDocService(db *sql.DB) *DocService {
	return &DocService{
		docs:     repo.NewDocRepo(db),
		projects: repo.NewProjectRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *DocService) Create(ctx context.Context, projectAlias string, in models.CreateDocInput) (*models.Doc, error) {
	if in.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	p, err := s.projects.GetByAlias(ctx, projectAlias)
	if err != nil {
		return nil, fmt.Errorf("project %q: %w", projectAlias, err)
	}
	now := timeutil.Now()
	d := &models.Doc{
		ID:             ids.New(),
		ProjectID:      p.ID,
		Title:          in.Title,
		CurrentVersion: 1,
		Actor:          in.Actor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.docs.Insert(ctx, d); err != nil {
		return nil, fmt.Errorf("create doc: %w", err)
	}
	v := &models.DocVersion{
		ID:        ids.New(),
		DocID:     d.ID,
		Version:   1,
		Title:     in.Title,
		Body:      in.Body,
		Actor:     in.Actor,
		CreatedAt: now,
	}
	if err := s.docs.InsertVersion(ctx, v); err != nil {
		return nil, fmt.Errorf("create doc version: %w", err)
	}
	d.Version = v
	s.logActivity(ctx, p.ID, "doc", d.ID, "created",
		fmt.Sprintf("Created doc %q", in.Title), in.Actor)
	return d, nil
}

func (s *DocService) Get(ctx context.Context, id string) (*models.Doc, error) {
	d, err := s.docs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("doc not found: %w", err)
	}
	v, err := s.docs.GetVersion(ctx, id, d.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("doc version: %w", err)
	}
	d.Version = v
	return d, nil
}

func (s *DocService) List(ctx context.Context, projectAlias string) ([]*models.Doc, error) {
	if projectAlias == "" {
		return s.docs.ListAcrossProjects(ctx, false)
	}
	p, err := s.projects.GetByAlias(ctx, projectAlias)
	if err != nil {
		return nil, fmt.Errorf("project %q: %w", projectAlias, err)
	}
	return s.docs.ListForProject(ctx, p.ID, false)
}

func (s *DocService) Update(ctx context.Context, id string, in models.UpdateDocInput) (*models.Doc, error) {
	d, err := s.docs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("doc not found: %w", err)
	}
	newVersion := d.CurrentVersion + 1
	title := in.Title
	if title == "" {
		title = d.Title
	}
	now := timeutil.Now()
	v := &models.DocVersion{
		ID:         ids.New(),
		DocID:      d.ID,
		Version:    newVersion,
		Title:      title,
		Body:       in.Body,
		Actor:      in.Actor,
		ChangeNote: in.ChangeNote,
		CreatedAt:  now,
	}
	if err := s.docs.InsertVersion(ctx, v); err != nil {
		return nil, fmt.Errorf("insert doc version: %w", err)
	}
	if err := s.docs.BumpVersion(ctx, id, newVersion, now, title); err != nil {
		return nil, fmt.Errorf("bump doc version: %w", err)
	}
	d.CurrentVersion = newVersion
	d.Title = title
	d.UpdatedAt = now
	d.Version = v
	s.logActivity(ctx, d.ProjectID, "doc", d.ID, "version_added",
		fmt.Sprintf("Added doc version v%d %q", newVersion, title), in.Actor)
	return d, nil
}

func (s *DocService) Append(ctx context.Context, id, input, changeNote string, actor models.Actor) (*models.Doc, error) {
	if input == "" {
		return nil, fmt.Errorf("content is required")
	}
	d, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	currentBody := ""
	if d.Version != nil {
		currentBody = d.Version.Body
	}
	return s.Update(ctx, id, models.UpdateDocInput{
		Body:       joinWithNewline(currentBody, input),
		ChangeNote: changeNote,
		Actor:      actor,
	})
}

func (s *DocService) History(ctx context.Context, id string) ([]*models.DocVersion, error) {
	if _, err := s.docs.GetByID(ctx, id); err != nil {
		return nil, fmt.Errorf("doc not found: %w", err)
	}
	return s.docs.ListVersions(ctx, id)
}

func (s *DocService) Delete(ctx context.Context, id string, actor models.Actor) error {
	d, err := s.docs.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("doc not found: %w", err)
	}
	s.logActivity(ctx, d.ProjectID, "doc", d.ID, "deleted",
		fmt.Sprintf("Deleted doc %q", d.Title), actor)
	return s.docs.Delete(ctx, id)
}

func (s *DocService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
