package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/models"
)

type LabelRepo struct{ db *sql.DB }

func NewLabelRepo(db *sql.DB) *LabelRepo { return &LabelRepo{db: db} }

func (r *LabelRepo) Insert(ctx context.Context, l *models.Label) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO labels(id,project_id,name,color) VALUES(?,?,?,?)`,
		l.ID, l.ProjectID, l.Name, l.Color)
	return err
}

func (r *LabelRepo) GetByID(ctx context.Context, id string) (*models.Label, error) {
	l := &models.Label{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,project_id,name,color FROM labels WHERE id=?`, id).
		Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("label not found")
	}
	return l, err
}

func (r *LabelRepo) GetByName(ctx context.Context, projectID, name string) (*models.Label, error) {
	l := &models.Label{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,project_id,name,color FROM labels WHERE project_id=? AND name=?`, projectID, name).
		Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

func (r *LabelRepo) ListForProject(ctx context.Context, projectID string) ([]*models.Label, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,project_id,name,color FROM labels WHERE project_id=? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Label
	for rows.Next() {
		l := &models.Label{}
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *LabelRepo) ListForTask(ctx context.Context, taskID string) ([]*models.Label, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.id,l.project_id,l.name,l.color FROM labels l
		 JOIN task_labels tl ON tl.label_id=l.id WHERE tl.task_id=? ORDER BY l.name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Label
	for rows.Next() {
		l := &models.Label{}
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *LabelRepo) Attach(ctx context.Context, taskID, labelID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO task_labels(task_id,label_id) VALUES(?,?)`, taskID, labelID)
	return err
}

func (r *LabelRepo) Detach(ctx context.Context, taskID, labelID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM task_labels WHERE task_id=? AND label_id=?`, taskID, labelID)
	return err
}

func (r *LabelRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM labels WHERE id=?`, id)
	return err
}
