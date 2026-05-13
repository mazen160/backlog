package repo

import (
	"context"
	"database/sql"

	"github.com/mazen160/backlog/internal/models"
)

type MemoryRepo struct{ db *sql.DB }

func NewMemoryRepo(db *sql.DB) *MemoryRepo { return &MemoryRepo{db: db} }

func (r *MemoryRepo) Insert(ctx context.Context, m *models.Memory) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_memory(id,project_id,body,tags,actor_kind,actor_name,created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		m.ID, m.ProjectID, m.Body, m.Tags, m.Actor.Kind, m.Actor.Name, m.CreatedAt)
	return err
}

func (r *MemoryRepo) ListForProject(ctx context.Context, projectID, tag string) ([]*models.Memory, error) {
	q := `SELECT id,project_id,body,tags,actor_kind,actor_name,created_at
	      FROM project_memory WHERE project_id=?`
	args := []interface{}{projectID}
	if tag != "" {
		q += ` AND (',' || tags || ',') LIKE ('%,' || ? || ',%')`
		args = append(args, tag)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Memory
	for rows.Next() {
		m := &models.Memory{}
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Body, &m.Tags,
			&m.Actor.Kind, &m.Actor.Name, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// ListAcrossProjects returns memory entries for every (non-archived) project,
// with each entry's owning project populated via the Project field so the client
// can render a project chip per row.
func (r *MemoryRepo) ListAcrossProjects(ctx context.Context, tag string) ([]*models.Memory, error) {
	q := `SELECT m.id, m.project_id, m.body, m.tags, m.actor_kind, m.actor_name, m.created_at,
	             p.alias, p.name
	      FROM project_memory m
	      JOIN projects p ON p.id=m.project_id
	      WHERE p.archived_at IS NULL`
	args := []interface{}{}
	if tag != "" {
		q += ` AND (',' || m.tags || ',') LIKE ('%,' || ? || ',%')`
		args = append(args, tag)
	}
	q += ` ORDER BY m.created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Memory
	for rows.Next() {
		m := &models.Memory{}
		var alias, name sql.NullString
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Body, &m.Tags,
			&m.Actor.Kind, &m.Actor.Name, &m.CreatedAt,
			&alias, &name); err != nil {
			return nil, err
		}
		if alias.Valid {
			m.Project = &models.Project{ID: m.ProjectID, Alias: alias.String, Name: name.String}
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *MemoryRepo) GetByID(ctx context.Context, id string) (*models.Memory, error) {
	m := &models.Memory{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,project_id,body,tags,actor_kind,actor_name,created_at
		 FROM project_memory WHERE id=?`, id).
		Scan(&m.ID, &m.ProjectID, &m.Body, &m.Tags, &m.Actor.Kind, &m.Actor.Name, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *MemoryRepo) UpdateBody(ctx context.Context, id, body string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_memory SET body=? WHERE id=?`, body, id)
	return err
}

func (r *MemoryRepo) UpdateBodyAndTags(ctx context.Context, id, body, tags string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_memory SET body=?, tags=? WHERE id=?`, body, tags, id)
	return err
}

func (r *MemoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM project_memory WHERE id=?`, id)
	return err
}
