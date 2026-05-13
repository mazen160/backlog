package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/models"
)

type CommentRepo struct{ db *sql.DB }

func NewCommentRepo(db *sql.DB) *CommentRepo { return &CommentRepo{db: db} }

func (r *CommentRepo) Insert(ctx context.Context, c *models.Comment) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO comments(id,task_id,body,actor_kind,actor_name,created_at) VALUES(?,?,?,?,?,?)`,
		c.ID, c.TaskID, c.Body, c.Actor.Kind, c.Actor.Name, c.CreatedAt)
	return err
}

func (r *CommentRepo) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	c := &models.Comment{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id,task_id,body,actor_kind,actor_name,created_at FROM comments WHERE id=?`, id).
		Scan(&c.ID, &c.TaskID, &c.Body, &c.Actor.Kind, &c.Actor.Name, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("comment not found")
	}
	return c, err
}

func (r *CommentRepo) ListForTask(ctx context.Context, taskID string) ([]*models.Comment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,task_id,body,actor_kind,actor_name,created_at FROM comments WHERE task_id=? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*models.Comment
	for rows.Next() {
		c := &models.Comment{}
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Body, &c.Actor.Kind, &c.Actor.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *CommentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id=?`, id)
	return err
}
