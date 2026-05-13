package service

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/timeutil"
)

const maxAttachmentSize = 10 * 1024 * 1024 // 10 MB

type AttachmentService struct {
	attachments *repo.AttachmentRepo
	activity    *repo.ActivityRepo
}

func NewAttachmentService(db *sql.DB) *AttachmentService {
	return &AttachmentService{
		attachments: repo.NewAttachmentRepo(db),
		activity:    repo.NewActivityRepo(db),
	}
}

func (s *AttachmentService) Add(ctx context.Context, name string, data []byte, linkedType, linkedID string, actor models.Actor) (*models.Attachment, error) {
	if len(data) > maxAttachmentSize {
		return nil, fmt.Errorf("attachment exceeds max size of %d MB", maxAttachmentSize/1024/1024)
	}
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	a := &models.AttachmentWithData{
		Attachment: models.Attachment{
			ID:         ids.New(),
			Name:       name,
			MimeType:   mimeType,
			Size:       int64(len(data)),
			LinkedType: linkedType,
			LinkedID:   linkedID,
			Actor:      actor,
			CreatedAt:  timeutil.Now(),
		},
		Data: data,
	}
	if err := s.attachments.Insert(ctx, a); err != nil {
		return nil, fmt.Errorf("add attachment: %w", err)
	}
	s.logActivity(ctx, "", "attachment", a.ID, "created",
		fmt.Sprintf("Added attachment %q to %s %s", name, linkedType, linkedID[:8]), actor)
	return &a.Attachment, nil
}

func (s *AttachmentService) Get(ctx context.Context, id string) (*models.AttachmentWithData, error) {
	a, err := s.attachments.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("attachment not found: %w", err)
	}
	return a, nil
}

func (s *AttachmentService) List(ctx context.Context, linkedType, linkedID string) ([]*models.Attachment, error) {
	return s.attachments.ListForLinked(ctx, linkedType, linkedID)
}

func (s *AttachmentService) ListForProject(ctx context.Context, projectAlias string) ([]*models.AttachmentMeta, error) {
	return s.attachments.ListForProject(ctx, projectAlias)
}

func (s *AttachmentService) Delete(ctx context.Context, id string, actor models.Actor) error {
	a, err := s.attachments.GetMeta(ctx, id)
	if err != nil {
		return fmt.Errorf("attachment not found: %w", err)
	}
	if err := s.attachments.Delete(ctx, id); err != nil {
		return err
	}
	s.logActivity(ctx, "", "attachment", id, "deleted",
		fmt.Sprintf("Deleted attachment %q", a.Name), actor)
	return nil
}

func (s *AttachmentService) logActivity(ctx context.Context, projectID, entity, entityID, action, summary string, actor models.Actor) {
	writeActivity(ctx, s.activity, projectID, entity, entityID, action, summary, actor)
}
