package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mazen160/backlog/internal/models"
)

type TaskRepo struct{ db *sql.DB }

func NewTaskRepo(db *sql.DB) *TaskRepo { return &TaskRepo{db: db} }

const taskCols = `t.id,t.project_id,p.alias,p.name,t.title,t.description,t.type,t.status,t.priority,
t.assignee,t.due_at,t.actor_kind,t.actor_name,t.source,t.external_ref,
t.completed_at,t.archived_at,t.created_at,t.updated_at,COALESCE(t.task_seq,0),COALESCE(t.project_path,'')`

const taskFrom = ` FROM tasks t LEFT JOIN projects p ON p.id=t.project_id`

func (r *TaskRepo) Insert(ctx context.Context, t *models.Task) error {
	return r.InsertWith(ctx, r.db, t)
}

// InsertWith inserts a task using the supplied Runner (either *sql.DB or *sql.Tx),
// so callers can run the insert inside a larger transaction. The task_seq is
// computed and assigned in a single SQL statement so concurrent inserts cannot
// collide on the same sequence number.
func (r *TaskRepo) InsertWith(ctx context.Context, runner Runner, t *models.Task) error {
	row := runner.QueryRowContext(ctx,
		`INSERT INTO tasks(id,project_id,title,description,type,status,priority,assignee,
		due_at,actor_kind,actor_name,source,external_ref,completed_at,archived_at,created_at,updated_at,task_seq,project_path)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
		       (SELECT COALESCE(MAX(task_seq),0)+1 FROM tasks),?)
		RETURNING task_seq`,
		t.ID, t.ProjectID, t.Title, t.Description, t.Type, t.Status, t.Priority,
		t.Assignee, nullInt64(t.DueAt), t.Actor.Kind, t.Actor.Name,
		t.Source, t.ExternalRef, nullInt64(t.CompletedAt), nullInt64(t.ArchivedAt),
		t.CreatedAt, t.UpdatedAt, t.ProjectPath)
	return row.Scan(&t.Seq)
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+taskCols+taskFrom+` WHERE t.id=?`, id)
	return r.scanOne(row)
}

func (r *TaskRepo) GetBySeq(ctx context.Context, seq int) (*models.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+taskCols+taskFrom+` WHERE t.task_seq=?`, seq)
	return r.scanOne(row)
}

func (r *TaskRepo) List(ctx context.Context, f models.TaskFilter) ([]*models.Task, error) {
	where, args := buildTaskFilter(f)
	q := `SELECT ` + taskCols + taskFrom + where + ` ORDER BY ` + taskSortClause(f.Sort)
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d OFFSET %d`, f.Limit, f.Offset)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *TaskRepo) ListBySearch(ctx context.Context, f models.TaskFilter) ([]*models.Task, error) {
	filterWhere, args := buildTaskFilter(f)
	// Combine FTS rowid filter with any other filters into a single WHERE clause
	ftsCondition := `t.rowid IN (SELECT rowid FROM tasks_fts WHERE tasks_fts MATCH ?)`
	var finalWhere string
	if filterWhere == "" {
		finalWhere = " WHERE " + ftsCondition
	} else {
		// filterWhere starts with " WHERE "; strip it and AND our condition first
		finalWhere = " WHERE " + ftsCondition + " AND " + filterWhere[7:]
	}
	q := `SELECT ` + taskCols + taskFrom + finalWhere + ` ORDER BY ` + taskSortClause(f.Sort)
	allArgs := append([]interface{}{sanitizeFTSQuery(f.Search)}, args...)
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d OFFSET %d`, f.Limit, f.Offset)
	}
	rows, err := r.db.QueryContext(ctx, q, allArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *TaskRepo) Count(ctx context.Context, f models.TaskFilter) (int, error) {
	where, args := buildTaskFilter(f)
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks t`+where, args...).Scan(&n)
	return n, err
}

func (r *TaskRepo) Update(ctx context.Context, t *models.Task) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET title=?,description=?,type=?,status=?,priority=?,assignee=?,
		due_at=?,source=?,external_ref=?,completed_at=?,archived_at=?,updated_at=?,project_path=? WHERE id=?`,
		t.Title, t.Description, t.Type, t.Status, t.Priority, t.Assignee,
		nullInt64(t.DueAt), t.Source, t.ExternalRef,
		nullInt64(t.CompletedAt), nullInt64(t.ArchivedAt), t.UpdatedAt, t.ProjectPath, t.ID)
	return err
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id=?`, id)
	return err
}

// sanitizeFTSQuery turns user-supplied search text into an FTS5-safe MATCH
// expression. Tokens that already use FTS5 operators (boolean keywords, prefix
// *, column filters, quoted phrases, parens) are passed through so power users
// keep `jwt OR csrf`, `sql*`, and `"exact phrase"`. Everything else is
// double-quoted as a phrase, which prevents bare punctuation or stray quotes
// from triggering an FTS5 syntax error that would leak as a raw SQL message.
func sanitizeFTSQuery(s string) string {
	s = strings.TrimSpace(s)
	// Trim trailing hyphens which can cause FTS5 phrase-match errors
	s = strings.TrimRight(s, "- ")
	if s == "" {
		return ""
	}
	if containsFTSOperator(s) {
		return s
	}
	// Wrap as a single phrase and double any embedded double-quotes per FTS5
	// quoting rules.
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func containsFTSOperator(s string) bool {
	if strings.ContainsAny(s, `"*():`) {
		return true
	}
	for _, kw := range []string{" AND ", " OR ", " NOT ", " NEAR "} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func taskSortClause(sort string) string {
	switch sort {
	case "created":
		return "t.created_at DESC"
	case "updated":
		return "t.updated_at DESC"
	case "seq":
		return "t.task_seq ASC"
	case "title":
		return "t.title ASC"
	default:
		return "t.priority ASC, t.created_at DESC"
	}
}

func buildTaskFilter(f models.TaskFilter) (string, []interface{}) {
	var conds []string
	var args []interface{}

	if f.ProjectAlias != "" {
		conds = append(conds, `t.project_id = (SELECT id FROM projects WHERE alias=?)`)
		args = append(args, f.ProjectAlias)
	}
	if f.Status != "" {
		conds = append(conds, `t.status=?`)
		args = append(args, f.Status)
	}
	if f.Type != "" {
		conds = append(conds, `t.type=?`)
		args = append(args, f.Type)
	}
	if f.Priority > 0 {
		conds = append(conds, `t.priority=?`)
		args = append(args, f.Priority)
	}
	if f.Assignee != "" {
		conds = append(conds, `t.assignee=?`)
		args = append(args, f.Assignee)
	}
	if f.ActorKind != "" {
		conds = append(conds, `t.actor_kind=?`)
		args = append(args, f.ActorKind)
	}
	if f.ActorName != "" {
		conds = append(conds, `t.actor_name=?`)
		args = append(args, f.ActorName)
	}
	if f.Source != "" {
		conds = append(conds, `t.source=?`)
		args = append(args, f.Source)
	}
	if !f.IncludeArchived {
		conds = append(conds, `t.archived_at IS NULL`)
	}
	if len(f.Labels) > 0 {
		placeholders := strings.Repeat("?,", len(f.Labels))
		placeholders = placeholders[:len(placeholders)-1]
		conds = append(conds, fmt.Sprintf(`t.id IN (SELECT tl.task_id FROM task_labels tl JOIN labels l ON l.id=tl.label_id WHERE l.name IN (%s))`, placeholders))
		for _, l := range f.Labels {
			args = append(args, l)
		}
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (r *TaskRepo) scanOne(row *sql.Row) (*models.Task, error) {
	t := &models.Task{Project: &models.Project{}}
	var dueAt, completedAt, archivedAt sql.NullInt64
	var projAlias, projName sql.NullString
	err := row.Scan(&t.ID, &t.ProjectID, &projAlias, &projName,
		&t.Title, &t.Description, &t.Type, &t.Status,
		&t.Priority, &t.Assignee, &dueAt, &t.Actor.Kind, &t.Actor.Name,
		&t.Source, &t.ExternalRef, &completedAt, &archivedAt, &t.CreatedAt, &t.UpdatedAt, &t.Seq, &t.ProjectPath)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, err
	}
	t.DueAt = scanNullInt64(dueAt)
	t.CompletedAt = scanNullInt64(completedAt)
	t.ArchivedAt = scanNullInt64(archivedAt)
	if projAlias.Valid {
		t.Project = &models.Project{ID: t.ProjectID, Alias: projAlias.String, Name: projName.String}
	}
	return t, nil
}

func (r *TaskRepo) scanRows(rows *sql.Rows) ([]*models.Task, error) {
	var result []*models.Task
	for rows.Next() {
		t := &models.Task{}
		var dueAt, completedAt, archivedAt sql.NullInt64
		var projAlias, projName sql.NullString
		if err := rows.Scan(&t.ID, &t.ProjectID, &projAlias, &projName,
			&t.Title, &t.Description, &t.Type, &t.Status,
			&t.Priority, &t.Assignee, &dueAt, &t.Actor.Kind, &t.Actor.Name,
			&t.Source, &t.ExternalRef, &completedAt, &archivedAt, &t.CreatedAt, &t.UpdatedAt, &t.Seq, &t.ProjectPath); err != nil {
			return nil, err
		}
		t.DueAt = scanNullInt64(dueAt)
		t.CompletedAt = scanNullInt64(completedAt)
		t.ArchivedAt = scanNullInt64(archivedAt)
		if projAlias.Valid {
			t.Project = &models.Project{ID: t.ProjectID, Alias: projAlias.String, Name: projName.String}
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
