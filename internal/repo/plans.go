package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/models"
)

type PlanRepo struct{ db *sql.DB }

func NewPlanRepo(db *sql.DB) *PlanRepo { return &PlanRepo{db: db} }

func (r *PlanRepo) InsertPlan(ctx context.Context, p *models.Plan) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO plans(id,task_id,current_version,source,archived_at,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?)`,
		p.ID, p.TaskID, p.CurrentVersion, p.Source, nullInt64(p.ArchivedAt), p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PlanRepo) InsertVersion(ctx context.Context, v *models.PlanVersion) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO plan_versions(id,plan_id,version,title,body,actor_kind,actor_name,change_note,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		v.ID, v.PlanID, v.Version, v.Title, v.Body,
		v.Actor.Kind, v.Actor.Name, v.ChangeNote, v.CreatedAt)
	return err
}

func (r *PlanRepo) BumpVersion(ctx context.Context, planID string, newVersion int, updatedAt int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plans SET current_version=?,updated_at=? WHERE id=?`,
		newVersion, updatedAt, planID)
	return err
}

func (r *PlanRepo) GetPlan(ctx context.Context, id string) (*models.Plan, error) {
	p := &models.Plan{}
	var archivedAt sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id,task_id,current_version,source,archived_at,created_at,updated_at FROM plans WHERE id=?`, id).
		Scan(&p.ID, &p.TaskID, &p.CurrentVersion, &p.Source, &archivedAt, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plan not found")
	}
	if err != nil {
		return nil, err
	}
	p.ArchivedAt = scanNullInt64(archivedAt)
	return p, nil
}

func (r *PlanRepo) GetVersion(ctx context.Context, planID string, version int) (*models.PlanVersion, error) {
	v := &models.PlanVersion{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,plan_id,version,title,body,actor_kind,actor_name,change_note,created_at
		 FROM plan_versions WHERE plan_id=? AND version=?`, planID, version).
		Scan(&v.ID, &v.PlanID, &v.Version, &v.Title, &v.Body,
			&v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plan version not found")
	}
	return v, err
}

func (r *PlanRepo) ListPlansForTask(ctx context.Context, taskID string, includeArchived bool) ([]*models.Plan, error) {
	q := `SELECT p.id,p.task_id,p.current_version,p.source,p.archived_at,p.created_at,p.updated_at,
		v.id,v.plan_id,v.version,v.title,v.body,v.actor_kind,v.actor_name,v.change_note,v.created_at
		FROM plans p
		JOIN plan_versions v ON v.plan_id=p.id AND v.version=p.current_version
		WHERE p.task_id=?`
	if !includeArchived {
		q += ` AND p.archived_at IS NULL`
	}
	q += ` ORDER BY p.created_at`
	rows, err := r.db.QueryContext(ctx, q, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []*models.Plan
	for rows.Next() {
		p := &models.Plan{}
		v := &models.PlanVersion{}
		var archivedAt sql.NullInt64
		if err := rows.Scan(
			&p.ID, &p.TaskID, &p.CurrentVersion, &p.Source, &archivedAt, &p.CreatedAt, &p.UpdatedAt,
			&v.ID, &v.PlanID, &v.Version, &v.Title, &v.Body, &v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		p.ArchivedAt = scanNullInt64(archivedAt)
		p.Version = v
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *PlanRepo) ListVersionHistory(ctx context.Context, planID string) ([]*models.PlanVersion, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,plan_id,version,title,body,actor_kind,actor_name,change_note,created_at
		 FROM plan_versions WHERE plan_id=? ORDER BY version ASC`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []*models.PlanVersion
	for rows.Next() {
		v := &models.PlanVersion{}
		if err := rows.Scan(&v.ID, &v.PlanID, &v.Version, &v.Title, &v.Body,
			&v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// PlanWithTask is a Plan augmented with the parent task's seq number and
// (when the listing spans multiple projects) the owning project's alias/name
// so the client can render a project chip per row.
type PlanWithTask struct {
	*models.Plan
	TaskSeq      int    `json:"task_seq"`
	ProjectAlias string `json:"project_alias,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
}

// ListPlansForProject returns all non-archived plans for a project identified by alias,
// with each plan's parent task seq number included. Pass empty alias to list plans
// across all projects (project_alias / project_name are populated in that case).
func (r *PlanRepo) ListPlansForProject(ctx context.Context, projectAlias string) ([]*PlanWithTask, error) {
	q := `SELECT p.id, p.task_id, p.current_version, p.source, p.archived_at, p.created_at, p.updated_at,
		v.id, v.plan_id, v.version, v.title, v.body, v.actor_kind, v.actor_name, v.change_note, v.created_at,
		t.task_seq, pr.alias, pr.name
		FROM plans p
		JOIN plan_versions v ON v.plan_id=p.id AND v.version=p.current_version
		JOIN tasks t ON t.id=p.task_id
		JOIN projects pr ON pr.id=t.project_id
		WHERE p.archived_at IS NULL AND t.archived_at IS NULL`
	args := []interface{}{}
	if projectAlias != "" {
		q += ` AND pr.alias=?`
		args = append(args, projectAlias)
	}
	q += ` ORDER BY p.updated_at DESC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []*PlanWithTask
	for rows.Next() {
		p := &models.Plan{}
		v := &models.PlanVersion{}
		var archivedAt sql.NullInt64
		var taskSeq int
		var prAlias, prName sql.NullString
		if err := rows.Scan(
			&p.ID, &p.TaskID, &p.CurrentVersion, &p.Source, &archivedAt, &p.CreatedAt, &p.UpdatedAt,
			&v.ID, &v.PlanID, &v.Version, &v.Title, &v.Body, &v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt,
			&taskSeq, &prAlias, &prName,
		); err != nil {
			return nil, err
		}
		p.ArchivedAt = scanNullInt64(archivedAt)
		p.Version = v
		pwt := &PlanWithTask{Plan: p, TaskSeq: taskSeq}
		// Only emit project info when listing across all projects, so the
		// per-project payload stays unchanged.
		if projectAlias == "" {
			if prAlias.Valid {
				pwt.ProjectAlias = prAlias.String
			}
			if prName.Valid {
				pwt.ProjectName = prName.String
			}
		}
		plans = append(plans, pwt)
	}
	return plans, rows.Err()
}

func (r *PlanRepo) ArchivePlan(ctx context.Context, planID string, archivedAt int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE plans SET archived_at=?,updated_at=? WHERE id=?`,
		archivedAt, archivedAt, planID)
	return err
}

func (r *PlanRepo) DeletePlan(ctx context.Context, planID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM plans WHERE id=?`, planID)
	return err
}
