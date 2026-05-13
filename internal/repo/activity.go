package repo

import (
	"context"
	"database/sql"
	"strings"

	"github.com/mazen160/backlog/internal/models"
)

type ActivityRepo struct{ db *sql.DB }

func NewActivityRepo(db *sql.DB) *ActivityRepo { return &ActivityRepo{db: db} }

func (r *ActivityRepo) Insert(ctx context.Context, a *models.Activity) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO activity_log(id,project_id,entity,entity_id,action,summary,actor_kind,actor_name,payload,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ProjectID, a.Entity, a.EntityID, a.Action, a.Summary, a.Actor.Kind, a.Actor.Name, a.Payload, a.CreatedAt)
	return err
}

func (r *ActivityRepo) ListForEntity(ctx context.Context, entity, entityID string) ([]*models.Activity, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,project_id,entity,entity_id,action,summary,actor_kind,actor_name,payload,created_at
		 FROM activity_log WHERE entity=? AND entity_id=? ORDER BY created_at`, entity, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *ActivityRepo) ListRecent(ctx context.Context, limit int) ([]*models.Activity, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,project_id,entity,entity_id,action,summary,actor_kind,actor_name,payload,created_at
		 FROM activity_log ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

// List returns activity events filtered by projectID, entityKind, and/or actor with pagination.
func (r *ActivityRepo) List(ctx context.Context, projectID, entityKind, actorKind, actorName string, limit, offset int) ([]*models.Activity, error) {
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	if offset < 0 {
		offset = 0
	}
	var conds []string
	var args []interface{}
	if projectID != "" {
		conds = append(conds, "project_id=?")
		args = append(args, projectID)
	}
	if entityKind != "" {
		conds = append(conds, "entity=?")
		args = append(args, entityKind)
	}
	if actorKind != "" {
		conds = append(conds, "actor_kind=?")
		args = append(args, actorKind)
	}
	if actorName != "" {
		conds = append(conds, "actor_name=?")
		args = append(args, actorName)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,project_id,entity,entity_id,action,summary,actor_kind,actor_name,payload,created_at
		 FROM activity_log `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *ActivityRepo) scanRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*models.Activity, error) {
	var result []*models.Activity
	for rows.Next() {
		a := &models.Activity{}
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Entity, &a.EntityID, &a.Action, &a.Summary,
			&a.Actor.Kind, &a.Actor.Name, &a.Payload, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
