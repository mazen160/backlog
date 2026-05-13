package service

import (
	"context"

	"github.com/mazen160/backlog/internal/ids"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/timeutil"
)

// writeActivity is the canonical helper for inserting an activity_log row.
// Every per-service logActivity method delegates here so the shape and the
// best-effort "error is logged not propagated" behavior live in one place.
func writeActivity(ctx context.Context, ar *repo.ActivityRepo, projectID, entity, entityID, action, summary string, actor models.Actor) {
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
	_ = ar.Insert(ctx, a)
}
