package repo

import (
	"context"
	"database/sql"

	"github.com/mazen160/backlog/internal/models"
)

type AttachmentRepo struct{ db *sql.DB }

func NewAttachmentRepo(db *sql.DB) *AttachmentRepo { return &AttachmentRepo{db: db} }

func (r *AttachmentRepo) Insert(ctx context.Context, a *models.AttachmentWithData) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO attachments(id,name,mime_type,size,data,linked_type,linked_id,actor_kind,actor_name,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.MimeType, a.Size, a.Data,
		a.LinkedType, a.LinkedID, a.Actor.Kind, a.Actor.Name, a.CreatedAt)
	return err
}

func (r *AttachmentRepo) GetByID(ctx context.Context, id string) (*models.AttachmentWithData, error) {
	a := &models.AttachmentWithData{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,name,mime_type,size,data,linked_type,linked_id,actor_kind,actor_name,created_at
		 FROM attachments WHERE id=?`, id).
		Scan(&a.ID, &a.Name, &a.MimeType, &a.Size, &a.Data,
			&a.LinkedType, &a.LinkedID, &a.Actor.Kind, &a.Actor.Name, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AttachmentRepo) ListForLinked(ctx context.Context, linkedType, linkedID string) ([]*models.Attachment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,mime_type,size,linked_type,linked_id,actor_kind,actor_name,created_at
		 FROM attachments WHERE linked_type=? AND linked_id=? ORDER BY created_at DESC`,
		linkedType, linkedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Attachment
	for rows.Next() {
		a := &models.Attachment{}
		if err := rows.Scan(&a.ID, &a.Name, &a.MimeType, &a.Size,
			&a.LinkedType, &a.LinkedID, &a.Actor.Kind, &a.Actor.Name, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *AttachmentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM attachments WHERE id=?`, id)
	return err
}

// ListForProject returns all attachments linked to tasks or docs that belong to the given project alias.
// linkedTitle is populated with the task title or doc title for display purposes.
// Pass an empty alias to list attachments across all (non-archived) projects;
// in that case project_alias / project_name are populated on each row so the
// client can render a project chip.
func (r *AttachmentRepo) ListForProject(ctx context.Context, projectAlias string) ([]*models.AttachmentMeta, error) {
	// Single union over task- and doc-linked attachments. The alias filter is
	// applied via a reusable predicate that both sides honor; passing "" makes
	// it a no-op so we get cross-project results in one query.
	aliasPred := ""
	args := []interface{}{}
	if projectAlias != "" {
		aliasPred = " AND p.alias=?"
	}
	// Note: SQLite's UNION ALL + ORDER BY needs to reference the result-set
	// column position (or an alias) — qualified names like `a.created_at`
	// don't resolve. Use the position of the created_at column in the SELECT
	// list (9 here, 1-indexed).
	q := `SELECT a.id, a.name, a.mime_type, a.size, a.linked_type, a.linked_id,
		       a.actor_kind, a.actor_name, a.created_at,
		       t.task_seq, t.title, '' AS doc_title,
		       p.alias, p.name
		FROM attachments a
		JOIN tasks t ON a.linked_type='task' AND a.linked_id=t.id
		JOIN projects p ON t.project_id=p.id` + aliasPred + `
		UNION ALL
		SELECT a.id, a.name, a.mime_type, a.size, a.linked_type, a.linked_id,
		       a.actor_kind, a.actor_name, a.created_at,
		       0, '', d.title AS doc_title,
		       p.alias, p.name
		FROM attachments a
		JOIN project_docs d ON a.linked_type='doc' AND a.linked_id=d.id
		JOIN projects p ON d.project_id=p.id` + aliasPred + `
		ORDER BY 9 DESC`
	if projectAlias != "" {
		args = append(args, projectAlias, projectAlias)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.AttachmentMeta
	for rows.Next() {
		m := &models.AttachmentMeta{}
		var alias, name sql.NullString
		if err := rows.Scan(&m.ID, &m.Name, &m.MimeType, &m.Size,
			&m.LinkedType, &m.LinkedID, &m.Actor.Kind, &m.Actor.Name, &m.CreatedAt,
			&m.TaskSeq, &m.TaskTitle, &m.DocTitle,
			&alias, &name); err != nil {
			return nil, err
		}
		// Only emit project info when listing across all projects, so the
		// per-project payload stays unchanged.
		if projectAlias == "" {
			if alias.Valid {
				m.ProjectAlias = alias.String
			}
			if name.Valid {
				m.ProjectName = name.String
			}
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetMeta returns attachment metadata without loading the blob.
func (r *AttachmentRepo) GetMeta(ctx context.Context, id string) (*models.Attachment, error) {
	a := &models.Attachment{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,name,mime_type,size,linked_type,linked_id,actor_kind,actor_name,created_at
		 FROM attachments WHERE id=?`, id).
		Scan(&a.ID, &a.Name, &a.MimeType, &a.Size,
			&a.LinkedType, &a.LinkedID, &a.Actor.Kind, &a.Actor.Name, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}
