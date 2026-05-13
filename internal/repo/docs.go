package repo

import (
	"context"
	"database/sql"

	"github.com/mazen160/backlog/internal/models"
)

type DocRepo struct{ db *sql.DB }

func NewDocRepo(db *sql.DB) *DocRepo { return &DocRepo{db: db} }

const docCols = `d.id,d.project_id,d.title,d.current_version,d.actor_kind,d.actor_name,
	d.archived_at,d.created_at,d.updated_at`

func (r *DocRepo) scanDoc(row interface {
	Scan(...interface{}) error
}) (*models.Doc, error) {
	d := &models.Doc{}
	var archivedAt sql.NullInt64
	if err := row.Scan(&d.ID, &d.ProjectID, &d.Title, &d.CurrentVersion,
		&d.Actor.Kind, &d.Actor.Name, &archivedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	if archivedAt.Valid {
		d.ArchivedAt = &archivedAt.Int64
	}
	return d, nil
}

func (r *DocRepo) Insert(ctx context.Context, d *models.Doc) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_docs(id,project_id,title,current_version,actor_kind,actor_name,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		d.ID, d.ProjectID, d.Title, d.CurrentVersion,
		d.Actor.Kind, d.Actor.Name, d.CreatedAt, d.UpdatedAt)
	return err
}

func (r *DocRepo) InsertVersion(ctx context.Context, v *models.DocVersion) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_doc_versions(id,doc_id,version,title,body,actor_kind,actor_name,change_note,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		v.ID, v.DocID, v.Version, v.Title, v.Body,
		v.Actor.Kind, v.Actor.Name, v.ChangeNote, v.CreatedAt)
	return err
}

func (r *DocRepo) BumpVersion(ctx context.Context, docID string, version int, updatedAt int64, title string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE project_docs SET current_version=?,updated_at=?,title=? WHERE id=?`,
		version, updatedAt, title, docID)
	return err
}

func (r *DocRepo) GetByID(ctx context.Context, id string) (*models.Doc, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+docCols+` FROM project_docs d WHERE d.id=?`, id)
	return r.scanDoc(row)
}

func (r *DocRepo) ListForProject(ctx context.Context, projectID string, includeArchived bool) ([]*models.Doc, error) {
	q := `SELECT ` + docCols + ` FROM project_docs d WHERE d.project_id=?`
	if !includeArchived {
		q += ` AND d.archived_at IS NULL`
	}
	q += ` ORDER BY d.updated_at DESC`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Doc
	for rows.Next() {
		d, err := r.scanDoc(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ListAcrossProjects returns docs for every project (excluding archived projects),
// with each doc's owning project populated via the Project field so the client
// can render a project chip per row.
func (r *DocRepo) ListAcrossProjects(ctx context.Context, includeArchived bool) ([]*models.Doc, error) {
	q := `SELECT ` + docCols + `, p.alias, p.name
	      FROM project_docs d
	      JOIN projects p ON p.id=d.project_id
	      WHERE p.archived_at IS NULL`
	if !includeArchived {
		q += ` AND d.archived_at IS NULL`
	}
	q += ` ORDER BY d.updated_at DESC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Doc
	for rows.Next() {
		d := &models.Doc{}
		var archivedAt sql.NullInt64
		var alias, name sql.NullString
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Title, &d.CurrentVersion,
			&d.Actor.Kind, &d.Actor.Name, &archivedAt, &d.CreatedAt, &d.UpdatedAt,
			&alias, &name); err != nil {
			return nil, err
		}
		if archivedAt.Valid {
			d.ArchivedAt = &archivedAt.Int64
		}
		if alias.Valid {
			d.Project = &models.Project{ID: d.ProjectID, Alias: alias.String, Name: name.String}
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *DocRepo) GetVersion(ctx context.Context, docID string, version int) (*models.DocVersion, error) {
	v := &models.DocVersion{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,doc_id,version,title,body,actor_kind,actor_name,change_note,created_at
		 FROM project_doc_versions WHERE doc_id=? AND version=?`, docID, version).
		Scan(&v.ID, &v.DocID, &v.Version, &v.Title, &v.Body,
			&v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *DocRepo) ListVersions(ctx context.Context, docID string) ([]*models.DocVersion, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,doc_id,version,title,body,actor_kind,actor_name,change_note,created_at
		 FROM project_doc_versions WHERE doc_id=? ORDER BY version ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.DocVersion
	for rows.Next() {
		v := &models.DocVersion{}
		if err := rows.Scan(&v.ID, &v.DocID, &v.Version, &v.Title, &v.Body,
			&v.Actor.Kind, &v.Actor.Name, &v.ChangeNote, &v.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *DocRepo) Archive(ctx context.Context, id string, at int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_docs SET archived_at=? WHERE id=?`, at, id)
	return err
}

func (r *DocRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM project_docs WHERE id=?`, id)
	return err
}
