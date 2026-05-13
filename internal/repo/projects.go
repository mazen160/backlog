package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/models"
)

type ProjectRepo struct{ db *sql.DB }

func NewProjectRepo(db *sql.DB) *ProjectRepo { return &ProjectRepo{db: db} }

func (r *ProjectRepo) Insert(ctx context.Context, p *models.Project) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects(id,alias,name,description,repo_path,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?)`,
		p.ID, p.Alias, p.Name, p.Description, p.RepoPath, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+projectCols+` FROM projects WHERE id=?`, id))
}

func (r *ProjectRepo) GetByAlias(ctx context.Context, alias string) (*models.Project, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+projectCols+` FROM projects WHERE alias=?`, alias))
}

func (r *ProjectRepo) List(ctx context.Context, includeArchived bool) ([]*models.Project, error) {
	q := `SELECT ` + projectCols + ` FROM projects`
	if !includeArchived {
		q += ` WHERE archived_at IS NULL`
	}
	q += ` ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *ProjectRepo) Update(ctx context.Context, p *models.Project) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE projects SET alias=?,name=?,description=?,repo_path=?,archived_at=?,updated_at=? WHERE id=?`,
		p.Alias, p.Name, p.Description, p.RepoPath, nullInt64(p.ArchivedAt), p.UpdatedAt, p.ID)
	return err
}

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	return err
}

const projectCols = `id,alias,name,description,repo_path,archived_at,created_at,updated_at`

func (r *ProjectRepo) scanOne(row *sql.Row) (*models.Project, error) {
	p := &models.Project{}
	var archivedAt sql.NullInt64
	err := row.Scan(&p.ID, &p.Alias, &p.Name, &p.Description, &p.RepoPath,
		&archivedAt, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, err
	}
	p.ArchivedAt = scanNullInt64(archivedAt)
	return p, nil
}

func (r *ProjectRepo) scanRows(rows *sql.Rows) ([]*models.Project, error) {
	var result []*models.Project
	for rows.Next() {
		p := &models.Project{}
		var archivedAt sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Alias, &p.Name, &p.Description, &p.RepoPath,
			&archivedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.ArchivedAt = scanNullInt64(archivedAt)
		result = append(result, p)
	}
	return result, rows.Err()
}
