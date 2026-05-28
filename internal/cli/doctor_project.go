package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/service"
)

type projectDoctorReport struct {
	Project       string        `json:"project"`
	GeneratedAt   int64         `json:"generated_at"`
	StaleAfter    string        `json:"stale_after"`
	IssueCount    int           `json:"issue_count"`
	TotalDetected int           `json:"total_detected"`
	Truncated     bool          `json:"truncated"`
	Issues        []doctorIssue `json:"issues"`
}

type doctorIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Ref      string `json:"ref,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence,omitempty"`
}

func doctorProjectCmd() *cobra.Command {
	var projectAlias, staleAfter string
	var limit int
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Detect stale, orphaned, and weakly-closed project work",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectAlias, err := requireProjectOrDefault(cmd, projectAlias)
			if err != nil {
				return err
			}
			projSvc := service.NewProjectService(app.DB)
			project, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
			if err != nil {
				return fmt.Errorf("project %q: %w", projectAlias, err)
			}
			staleDuration, err := parseDurationFlag(staleAfter)
			if err != nil {
				return fmt.Errorf("--stale-after: %w", err)
			}
			report, err := buildProjectDoctorReport(cmd.Context(), app.DB, project.ID, project.Alias, staleAfter, time.Now().UTC().Add(-staleDuration).UnixNano(), limit)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(report)
				return nil
			}
			printProjectDoctorReport(report)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectAlias, "project", "", "project alias")
	cmd.Flags().StringVar(&staleAfter, "stale-after", "7d", "flag doing tasks with no updates after this duration")
	cmd.Flags().IntVar(&limit, "limit", 100, "max issues to return")
	return cmd
}

func buildProjectDoctorReport(ctx context.Context, db *sql.DB, projectID, projectAlias, staleAfter string, staleCutoff int64, limit int) (*projectDoctorReport, error) {
	if limit <= 0 {
		limit = 100
	}
	report := &projectDoctorReport{
		Project:     projectAlias,
		GeneratedAt: time.Now().UTC().UnixNano(),
		StaleAfter:  staleAfter,
	}
	detectLimit := 10000
	add := func(issue doctorIssue) {
		if len(report.Issues) < limit {
			report.Issues = append(report.Issues, issue)
		}
		report.TotalDetected++
	}
	if err := addTaskIssues(ctx, db, projectID, detectLimit, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='todo'
		  AND NOT EXISTS (
		    SELECT 1 FROM activity_log a
		    WHERE a.entity='task' AND a.entity_id=t.id AND a.action='status_changed'
		  )
		ORDER BY t.created_at ASC LIMIT ?`,
		func(ref taskIssueRef) doctorIssue {
			return doctorIssue{
				Severity: "warning",
				Code:     "task_created_never_started",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Task is still todo and has never had a status transition.",
				Evidence: "no task status_changed activity",
			}
		}, add); err != nil {
		return nil, err
	}
	if err := addTaskIssuesWithArg(ctx, db, projectID, staleCutoff, detectLimit, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='doing' AND t.updated_at<?
		ORDER BY t.updated_at ASC LIMIT ?`,
		func(ref taskIssueRef) doctorIssue {
			return doctorIssue{
				Severity: "warning",
				Code:     "stale_doing_task",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   fmt.Sprintf("Task is still doing and has not been updated within %s.", staleAfter),
				Evidence: fmt.Sprintf("updated_at=%s", formatNano(ref.UpdatedAt)),
			}
		}, add); err != nil {
		return nil, err
	}
	if err := addTaskIssues(ctx, db, projectID, detectLimit, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status IN ('todo','doing')
		  AND NOT EXISTS (SELECT 1 FROM plans p WHERE p.task_id=t.id AND p.archived_at IS NULL)
		ORDER BY t.created_at ASC LIMIT ?`,
		func(ref taskIssueRef) doctorIssue {
			return doctorIssue{
				Severity: "info",
				Code:     "task_without_plan",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Open task has no attached plan.",
			}
		}, add); err != nil {
		return nil, err
	}
	if err := addTaskIssues(ctx, db, projectID, detectLimit, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='done'
		  AND NOT EXISTS (SELECT 1 FROM comments c WHERE c.task_id=t.id)
		ORDER BY t.completed_at DESC LIMIT ?`,
		func(ref taskIssueRef) doctorIssue {
			return doctorIssue{
				Severity: "warning",
				Code:     "done_without_comments",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Task is done but has no comments.",
			}
		}, add); err != nil {
		return nil, err
	}
	if err := addTaskIssues(ctx, db, projectID, detectLimit, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='done'
		  AND NOT EXISTS (
		    SELECT 1 FROM comments c
		    WHERE c.task_id=t.id
		      AND (
		        lower(c.body) LIKE '%verified%' OR
		        lower(c.body) LIKE '%implemented%' OR
		        lower(c.body) LIKE '%fixed%' OR
		        lower(c.body) LIKE '%tested%' OR
		        lower(c.body) LIKE '%[worker receipt]%' OR
		        lower(c.body) LIKE '%[judge receipt]%'
		      )
		  )
		ORDER BY t.completed_at DESC LIMIT ?`,
		func(ref taskIssueRef) doctorIssue {
			return doctorIssue{
				Severity: "warning",
				Code:     "done_without_completion_evidence",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Task is done but comments do not contain recognizable completion evidence.",
				Evidence: "looked for verified/implemented/fixed/tested or receipt comments",
			}
		}, add); err != nil {
		return nil, err
	}
	labelOnly, err := latestActivityLabelOnlyIssues(ctx, db, projectID, detectLimit)
	if err != nil {
		return nil, err
	}
	for _, issue := range labelOnly {
		add(issue)
	}
	finalAuditIssues, err := finalAuditIntegrityIssues(ctx, db, projectID)
	if err != nil {
		return nil, err
	}
	for _, issue := range finalAuditIssues {
		add(issue)
	}
	report.IssueCount = len(report.Issues)
	report.Truncated = report.TotalDetected > report.IssueCount
	return report, nil
}

func addTaskIssues(ctx context.Context, db *sql.DB, projectID string, limit int, query string, build func(taskIssueRef) doctorIssue, add func(doctorIssue)) error {
	rows, err := db.QueryContext(ctx, query, projectID, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		ref, err := scanTaskIssueRef(rows)
		if err != nil {
			return err
		}
		add(build(ref))
	}
	return rows.Err()
}

func addTaskIssuesWithArg(ctx context.Context, db *sql.DB, projectID string, arg any, limit int, query string, build func(taskIssueRef) doctorIssue, add func(doctorIssue)) error {
	rows, err := db.QueryContext(ctx, query, projectID, arg, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		ref, err := scanTaskIssueRef(rows)
		if err != nil {
			return err
		}
		add(build(ref))
	}
	return rows.Err()
}

func latestActivityLabelOnlyIssues(ctx context.Context, db *sql.DB, projectID string, limit int) ([]doctorIssue, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT task_seq, id, title, status, type, updated_at, actor_kind, actor_name
		FROM tasks
		WHERE project_id=? AND archived_at IS NULL
		ORDER BY updated_at DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	var refs []taskIssueRef
	for rows.Next() {
		ref, err := scanTaskIssueRef(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, ref)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []doctorIssue
	for _, ref := range refs {
		var entity, summary string
		err = db.QueryRowContext(ctx, `
			SELECT entity, summary FROM activity_log
			WHERE project_id=? AND (
			  (entity='task' AND entity_id=?) OR
			  summary LIKE ?
			)
			ORDER BY created_at DESC LIMIT 1`, projectID, ref.ID, "%"+ref.ID[:8]+"%").Scan(&entity, &summary)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		if entity == "label" {
			out = append(out, doctorIssue{
				Severity: "info",
				Code:     "latest_activity_label_churn_only",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Latest task-related activity is label churn, not task progress.",
				Evidence: summary,
			})
		}
	}
	return out, nil
}

func finalAuditIntegrityIssues(ctx context.Context, db *sql.DB, projectID string) ([]doctorIssue, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name, COALESCE(t.completed_at, t.updated_at)
		FROM tasks t
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='done'
		  AND (
		    lower(t.title) LIKE '%final audit%' OR
		    t.id IN (
		      SELECT tl.task_id FROM task_labels tl
		      JOIN labels l ON l.id=tl.label_id
		      WHERE l.project_id=t.project_id AND l.name='final-audit'
		    )
		  )`, projectID)
	if err != nil {
		return nil, err
	}
	type finalAuditRef struct {
		taskIssueRef
		closedAt int64
	}
	var refs []finalAuditRef
	for rows.Next() {
		ref, closedAt, err := scanTaskIssueRefWithCompleted(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, finalAuditRef{taskIssueRef: ref, closedAt: closedAt})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []doctorIssue
	for _, ref := range refs {
		var openCount int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tasks
			WHERE project_id=? AND archived_at IS NULL AND id<>? AND status IN ('todo','doing') AND created_at<=?`,
			projectID, ref.ID, ref.closedAt).Scan(&openCount); err != nil {
			return nil, err
		}
		if openCount > 0 {
			out = append(out, doctorIssue{
				Severity: "error",
				Code:     "final_audit_closed_with_open_work",
				Ref:      ref.Ref,
				TaskID:   ref.ID,
				Title:    ref.Title,
				Detail:   "Final-audit task is done while child/project work remains open.",
				Evidence: fmt.Sprintf("%d open task(s) existed before the audit closed", openCount),
			})
		}
	}
	return out, nil
}

func scanTaskIssueRefWithCompleted(scanner interface{ Scan(...any) error }) (taskIssueRef, int64, error) {
	var seq int
	var actorKind, actorName string
	var closedAt int64
	ref := taskIssueRef{}
	if err := scanner.Scan(&seq, &ref.ID, &ref.Title, &ref.Status, &ref.Type, &ref.UpdatedAt, &actorKind, &actorName, &closedAt); err != nil {
		return ref, 0, err
	}
	ref.Ref = fmt.Sprintf("TASK-%d", seq)
	ref.Actor = actorLabel(actorKind, actorName)
	return ref, closedAt, nil
}

func printProjectDoctorReport(r *projectDoctorReport) {
	if r.Truncated {
		fmt.Fprintf(os.Stdout, "Project doctor for %s: %d issue(s) shown, %d detected\n", r.Project, r.IssueCount, r.TotalDetected)
	} else {
		fmt.Fprintf(os.Stdout, "Project doctor for %s: %d issue(s)\n", r.Project, r.IssueCount)
	}
	if r.TotalDetected == 0 {
		fmt.Fprintln(os.Stdout, "No stale, orphaned, or weakly-closed work detected.")
		return
	}
	for _, issue := range r.Issues {
		ref := issue.Ref
		if ref == "" {
			ref = "project"
		}
		fmt.Fprintf(os.Stdout, "- [%s] %s %s: %s", strings.ToUpper(issue.Severity), ref, issue.Code, issue.Detail)
		if issue.Evidence != "" {
			fmt.Fprintf(os.Stdout, " (%s)", issue.Evidence)
		}
		fmt.Fprintln(os.Stdout)
	}
	if r.Truncated {
		fmt.Fprintf(os.Stdout, "... %d more issue(s) not shown; increase --limit to inspect them.\n", r.TotalDetected-r.IssueCount)
	}
}
