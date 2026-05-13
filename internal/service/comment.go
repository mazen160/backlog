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

type CommentService struct {
	comments *repo.CommentRepo
	tasks    *repo.TaskRepo
	activity *repo.ActivityRepo
}

func NewCommentService(db *sql.DB) *CommentService {
	return &CommentService{
		comments: repo.NewCommentRepo(db),
		tasks:    repo.NewTaskRepo(db),
		activity: repo.NewActivityRepo(db),
	}
}

func (s *CommentService) Create(ctx context.Context, in models.CreateCommentInput) (*models.Comment, error) {
	if in.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if in.Body == "" {
		return nil, fmt.Errorf("body is required")
	}
	c := &models.Comment{
		ID:        ids.New(),
		TaskID:    in.TaskID,
		Body:      in.Body,
		Actor:     in.Actor,
		CreatedAt: timeutil.Now(),
	}
	if err := s.comments.Insert(ctx, c); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	// Derive projectID from task for activity log
	projectID := ""
	if t, err := s.tasks.GetByID(ctx, in.TaskID); err == nil {
		projectID = t.ProjectID
		s.logActivity(ctx, projectID, "comment", c.ID, "created",
			fmt.Sprintf("Added comment on TASK-%d", t.Seq), in.Actor)
	} else {
		s.logActivity(ctx, projectID, "comment", c.ID, "created",
			fmt.Sprintf("Added comment on task %s", in.TaskID[:8]), in.Actor)
	}
	return c, nil
}

func (s *CommentService) ListForTask(ctx context.Context, taskID string) ([]*models.Comment, error) {
	return s.comments.ListForTask(ctx, taskID)
}

func (s *CommentService) Delete(ctx context.Context, id string, actor models.Actor) error {
	c, err := s.comments.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.comments.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	projectID := ""
	if t, err := s.tasks.GetByID(ctx, c.TaskID); err == nil {
		projectID = t.ProjectID
	}
	s.logActivity(ctx, projectID, "comment", id, "deleted",
		fmt.Sprintf("Deleted comment %s", id[:8]), actor)
	return nil
}

func (s *CommentService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
