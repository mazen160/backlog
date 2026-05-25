package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/service"
)

type activityAnalysisReport struct {
	Project              string                   `json:"project"`
	Since                string                   `json:"since"`
	SinceAt              int64                    `json:"since_at"`
	GeneratedAt          int64                    `json:"generated_at"`
	Tasks                analysisTaskCounts       `json:"tasks"`
	CycleTimeByType      []analysisDurationByType `json:"cycle_time_by_type"`
	StatusLatency        analysisLatency          `json:"status_latency"`
	WIPByActor           []analysisActorCount     `json:"wip_by_actor"`
	NoCompletionEvidence []taskIssueRef           `json:"no_completion_evidence"`
	ReopenedTasks        []taskIssueRef           `json:"reopened_tasks"`
	BugFollowups         []taskIssueRef           `json:"bug_followups"`
	StaleDoingTasks      []taskIssueRef           `json:"stale_doing_tasks"`
	Duplicates           []duplicateSignal        `json:"duplicates"`
	TopLabelsByChurn     []labelChurnSignal       `json:"top_labels_by_churn"`
	HumanVsAICloses      []actorKindCount         `json:"human_vs_ai_closes"`
}

type analysisTaskCounts struct {
	Created   int `json:"created"`
	Completed int `json:"completed"`
	Todo      int `json:"todo"`
	Doing     int `json:"doing"`
	Done      int `json:"done"`
}

type analysisDurationByType struct {
	Type         string `json:"type"`
	Count        int    `json:"count"`
	Average      string `json:"average"`
	AverageNanos int64  `json:"average_nanos"`
	Minimum      string `json:"minimum"`
	MinimumNanos int64  `json:"minimum_nanos"`
	Maximum      string `json:"maximum"`
	MaximumNanos int64  `json:"maximum_nanos"`
}

type analysisLatency struct {
	TodoToDoingCount        int    `json:"todo_to_doing_count"`
	TodoToDoingAverage      string `json:"todo_to_doing_average"`
	TodoToDoingAverageNanos int64  `json:"todo_to_doing_average_nanos"`
	DoingToDoneCount        int    `json:"doing_to_done_count"`
	DoingToDoneAverage      string `json:"doing_to_done_average"`
	DoingToDoneAverageNanos int64  `json:"doing_to_done_average_nanos"`
}

type analysisActorCount struct {
	ActorKind string `json:"actor_kind"`
	ActorName string `json:"actor_name"`
	Actor     string `json:"actor"`
	Count     int    `json:"count"`
}

type taskIssueRef struct {
	Ref       string `json:"ref"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status,omitempty"`
	Type      string `json:"type,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Detail    string `json:"detail,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

type duplicateSignal struct {
	Kind    string `json:"kind"`
	Scope   string `json:"scope"`
	Value   string `json:"value"`
	Count   int    `json:"count"`
	TaskRef string `json:"task_ref,omitempty"`
}

type labelChurnSignal struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type actorKindCount struct {
	ActorKind string `json:"actor_kind"`
	Count     int    `json:"count"`
}

func activityAnalyzeCmd() *cobra.Command {
	var projectAlias, since, staleAfter string
	var limit int
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Summarize project activity and workflow health",
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
			now := time.Now().UTC()
			sinceAt, sinceLabel, err := parseSinceFlag(since, now)
			if err != nil {
				return err
			}
			staleDuration, err := parseDurationFlag(staleAfter)
			if err != nil {
				return fmt.Errorf("--stale-after: %w", err)
			}
			report, err := buildActivityAnalysis(cmd.Context(), app.DB, project.ID, project.Alias, sinceLabel, sinceAt, now.Add(-staleDuration).UnixNano(), limit)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(report)
				return nil
			}
			printActivityAnalysis(report)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectAlias, "project", "", "project alias")
	cmd.Flags().StringVar(&since, "since", "7d", "activity window: 7d, 24h, all, RFC3339, or YYYY-MM-DD")
	cmd.Flags().StringVar(&staleAfter, "stale-after", "7d", "flag doing tasks with no updates after this duration")
	cmd.Flags().IntVar(&limit, "limit", 10, "max task examples per section")
	return cmd
}

func buildActivityAnalysis(ctx context.Context, db *sql.DB, projectID, projectAlias, since string, sinceAt, staleCutoff int64, limit int) (*activityAnalysisReport, error) {
	if limit <= 0 {
		limit = 10
	}
	r := &activityAnalysisReport{
		Project:     projectAlias,
		Since:       since,
		SinceAt:     sinceAt,
		GeneratedAt: time.Now().UTC().UnixNano(),
	}
	var err error
	if r.Tasks.Created, err = countTasks(ctx, db, `project_id=? AND archived_at IS NULL AND created_at>=?`, projectID, sinceAt); err != nil {
		return nil, err
	}
	if r.Tasks.Completed, err = countTasks(ctx, db, `project_id=? AND archived_at IS NULL AND completed_at IS NOT NULL AND completed_at>=?`, projectID, sinceAt); err != nil {
		return nil, err
	}
	if r.Tasks.Todo, err = countTasks(ctx, db, `project_id=? AND archived_at IS NULL AND status='todo'`, projectID); err != nil {
		return nil, err
	}
	if r.Tasks.Doing, err = countTasks(ctx, db, `project_id=? AND archived_at IS NULL AND status='doing'`, projectID); err != nil {
		return nil, err
	}
	if r.Tasks.Done, err = countTasks(ctx, db, `project_id=? AND archived_at IS NULL AND status='done'`, projectID); err != nil {
		return nil, err
	}
	if r.CycleTimeByType, err = queryCycleTimeByType(ctx, db, projectID, sinceAt); err != nil {
		return nil, err
	}
	if r.StatusLatency, err = queryStatusLatency(ctx, db, projectID, sinceAt); err != nil {
		return nil, err
	}
	if r.WIPByActor, err = queryWIPByActor(ctx, db, projectID); err != nil {
		return nil, err
	}
	if r.NoCompletionEvidence, err = queryNoCompletionEvidence(ctx, db, projectID, sinceAt, limit); err != nil {
		return nil, err
	}
	if r.ReopenedTasks, err = queryActivityTaskExamples(ctx, db, projectID, sinceAt, limit, "a.action='status_changed' AND a.summary LIKE '% from done to %'"); err != nil {
		return nil, err
	}
	if r.BugFollowups, err = queryTaskExamples(ctx, db, projectID, limit, `t.archived_at IS NULL AND t.type='bug' AND t.created_at>=?`, sinceAt); err != nil {
		return nil, err
	}
	if r.StaleDoingTasks, err = queryTaskExamples(ctx, db, projectID, limit, `t.archived_at IS NULL AND t.status='doing' AND t.updated_at<?`, staleCutoff); err != nil {
		return nil, err
	}
	if r.Duplicates, err = queryDuplicateSignals(ctx, db, projectID, limit); err != nil {
		return nil, err
	}
	if r.TopLabelsByChurn, err = queryTopLabelsByChurn(ctx, db, projectID, sinceAt, limit); err != nil {
		return nil, err
	}
	if r.HumanVsAICloses, err = queryCloseRatio(ctx, db, projectID, sinceAt); err != nil {
		return nil, err
	}
	return r, nil
}

func countTasks(ctx context.Context, db *sql.DB, where string, args ...any) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&n)
	return n, err
}

func queryCycleTimeByType(ctx context.Context, db *sql.DB, projectID string, sinceAt int64) ([]analysisDurationByType, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT type, COUNT(*), AVG(completed_at-created_at), MIN(completed_at-created_at), MAX(completed_at-created_at)
		FROM tasks
		WHERE project_id=? AND archived_at IS NULL AND completed_at IS NOT NULL AND completed_at>=?
		GROUP BY type ORDER BY type`, projectID, sinceAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []analysisDurationByType
	for rows.Next() {
		var row analysisDurationByType
		var avg sql.NullFloat64
		if err := rows.Scan(&row.Type, &row.Count, &avg, &row.MinimumNanos, &row.MaximumNanos); err != nil {
			return nil, err
		}
		if avg.Valid {
			row.AverageNanos = int64(avg.Float64)
			row.Average = formatDurationNanos(row.AverageNanos)
		}
		row.Minimum = formatDurationNanos(row.MinimumNanos)
		row.Maximum = formatDurationNanos(row.MaximumNanos)
		out = append(out, row)
	}
	return out, rows.Err()
}

func queryStatusLatency(ctx context.Context, db *sql.DB, projectID string, sinceAt int64) (analysisLatency, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.created_at, a.summary, a.created_at
		FROM activity_log a
		JOIN tasks t ON t.id=a.entity_id
		WHERE a.project_id=? AND a.entity='task' AND a.action='status_changed' AND a.created_at>=?
		ORDER BY t.id, a.created_at`, projectID, sinceAt)
	if err != nil {
		return analysisLatency{}, err
	}
	defer rows.Close()
	type seen struct {
		created int64
		doingAt int64
	}
	byTask := map[string]seen{}
	var todoToDoing, doingToDone []int64
	for rows.Next() {
		var taskID, summary string
		var createdAt, eventAt int64
		if err := rows.Scan(&taskID, &createdAt, &summary, &eventAt); err != nil {
			return analysisLatency{}, err
		}
		s := byTask[taskID]
		if s.created == 0 {
			s.created = createdAt
		}
		if strings.Contains(summary, " from todo to doing") {
			if eventAt >= createdAt {
				todoToDoing = append(todoToDoing, eventAt-createdAt)
			}
			if s.doingAt == 0 {
				s.doingAt = eventAt
			}
		}
		if strings.Contains(summary, " to done") && s.doingAt > 0 && eventAt >= s.doingAt {
			doingToDone = append(doingToDone, eventAt-s.doingAt)
		}
		byTask[taskID] = s
	}
	if err := rows.Err(); err != nil {
		return analysisLatency{}, err
	}
	tdCount, tdAvg := averageNanos(todoToDoing)
	ddCount, ddAvg := averageNanos(doingToDone)
	return analysisLatency{
		TodoToDoingCount:        tdCount,
		TodoToDoingAverage:      formatDurationNanos(tdAvg),
		TodoToDoingAverageNanos: tdAvg,
		DoingToDoneCount:        ddCount,
		DoingToDoneAverage:      formatDurationNanos(ddAvg),
		DoingToDoneAverageNanos: ddAvg,
	}, nil
}

func averageNanos(values []int64) (int, int64) {
	if len(values) == 0 {
		return 0, 0
	}
	var total int64
	for _, v := range values {
		total += v
	}
	return len(values), total / int64(len(values))
}

func queryWIPByActor(ctx context.Context, db *sql.DB, projectID string) ([]analysisActorCount, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT actor_kind, actor_name, COUNT(*)
		FROM tasks
		WHERE project_id=? AND archived_at IS NULL AND status='doing'
		GROUP BY actor_kind, actor_name
		ORDER BY COUNT(*) DESC, actor_kind, actor_name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []analysisActorCount
	for rows.Next() {
		var row analysisActorCount
		if err := rows.Scan(&row.ActorKind, &row.ActorName, &row.Count); err != nil {
			return nil, err
		}
		row.Actor = actorLabel(row.ActorKind, row.ActorName)
		out = append(out, row)
	}
	return out, rows.Err()
}

func queryNoCompletionEvidence(ctx context.Context, db *sql.DB, projectID string, sinceAt int64, limit int) ([]taskIssueRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		LEFT JOIN comments c ON c.task_id=t.id
		LEFT JOIN plans p ON p.task_id=t.id AND p.archived_at IS NULL
		WHERE t.project_id=? AND t.archived_at IS NULL AND t.status='done'
		  AND t.completed_at IS NOT NULL AND t.completed_at>=?
		GROUP BY t.id, t.task_seq, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		HAVING COUNT(DISTINCT c.id)=0 AND COUNT(DISTINCT p.id)=0
		ORDER BY t.completed_at DESC LIMIT ?`, projectID, sinceAt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTaskIssueRefs(rows, "done without comments or plans")
}

func queryActivityTaskExamples(ctx context.Context, db *sql.DB, projectID string, sinceAt int64, limit int, predicate string) ([]taskIssueRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name, a.summary
		FROM activity_log a
		JOIN tasks t ON t.id=a.entity_id
		WHERE a.project_id=? AND a.entity='task' AND a.created_at>=? AND `+predicate+`
		ORDER BY a.created_at DESC LIMIT ?`, projectID, sinceAt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []taskIssueRef
	for rows.Next() {
		ref, err := scanTaskIssueRefWithDetail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func queryTaskExamples(ctx context.Context, db *sql.DB, projectID string, limit int, predicate string, args ...any) ([]taskIssueRef, error) {
	allArgs := append([]any{projectID}, args...)
	allArgs = append(allArgs, limit)
	rows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, t.id, t.title, t.status, t.type, t.updated_at, t.actor_kind, t.actor_name
		FROM tasks t
		WHERE t.project_id=? AND `+predicate+`
		ORDER BY t.updated_at DESC LIMIT ?`, allArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTaskIssueRefs(rows, "")
}

func scanTaskIssueRefs(rows *sql.Rows, detail string) ([]taskIssueRef, error) {
	var out []taskIssueRef
	for rows.Next() {
		ref, err := scanTaskIssueRef(rows)
		if err != nil {
			return nil, err
		}
		ref.Detail = detail
		out = append(out, ref)
	}
	return out, rows.Err()
}

func scanTaskIssueRef(scanner interface{ Scan(...any) error }) (taskIssueRef, error) {
	var seq int
	var actorKind, actorName string
	ref := taskIssueRef{}
	if err := scanner.Scan(&seq, &ref.ID, &ref.Title, &ref.Status, &ref.Type, &ref.UpdatedAt, &actorKind, &actorName); err != nil {
		return ref, err
	}
	ref.Ref = fmt.Sprintf("TASK-%d", seq)
	ref.Actor = actorLabel(actorKind, actorName)
	return ref, nil
}

func scanTaskIssueRefWithDetail(scanner interface{ Scan(...any) error }) (taskIssueRef, error) {
	var seq int
	var actorKind, actorName string
	ref := taskIssueRef{}
	if err := scanner.Scan(&seq, &ref.ID, &ref.Title, &ref.Status, &ref.Type, &ref.UpdatedAt, &actorKind, &actorName, &ref.Detail); err != nil {
		return ref, err
	}
	ref.Ref = fmt.Sprintf("TASK-%d", seq)
	ref.Actor = actorLabel(actorKind, actorName)
	return ref, nil
}

func queryDuplicateSignals(ctx context.Context, db *sql.DB, projectID string, limit int) ([]duplicateSignal, error) {
	var out []duplicateSignal
	labelRows, err := db.QueryContext(ctx, `
		SELECT lower(name), group_concat(name, ', '), COUNT(*)
		FROM labels WHERE project_id=?
		GROUP BY lower(name) HAVING COUNT(*)>1
		ORDER BY COUNT(*) DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	for labelRows.Next() {
		var normalized, names string
		var count int
		if err := labelRows.Scan(&normalized, &names, &count); err != nil {
			labelRows.Close()
			return nil, err
		}
		out = append(out, duplicateSignal{Kind: "label", Scope: "project", Value: names, Count: count})
		_ = normalized
	}
	if err := labelRows.Close(); err != nil {
		return nil, err
	}

	planRows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, pv.title, COUNT(*)
		FROM plans p
		JOIN plan_versions pv ON pv.plan_id=p.id AND pv.version=p.current_version
		JOIN tasks t ON t.id=p.task_id
		WHERE t.project_id=? AND p.archived_at IS NULL
		GROUP BY t.id, lower(pv.title) HAVING COUNT(*)>1
		ORDER BY COUNT(*) DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	for planRows.Next() {
		var seq, count int
		var title string
		if err := planRows.Scan(&seq, &title, &count); err != nil {
			planRows.Close()
			return nil, err
		}
		out = append(out, duplicateSignal{Kind: "plan", Scope: fmt.Sprintf("TASK-%d", seq), Value: title, Count: count, TaskRef: fmt.Sprintf("TASK-%d", seq)})
	}
	if err := planRows.Close(); err != nil {
		return nil, err
	}

	commentRows, err := db.QueryContext(ctx, `
		SELECT t.task_seq, substr(c.body, 1, 80), COUNT(*)
		FROM comments c
		JOIN tasks t ON t.id=c.task_id
		WHERE t.project_id=?
		GROUP BY t.id, c.body HAVING COUNT(*)>1
		ORDER BY COUNT(*) DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer commentRows.Close()
	for commentRows.Next() {
		var seq, count int
		var body string
		if err := commentRows.Scan(&seq, &body, &count); err != nil {
			return nil, err
		}
		out = append(out, duplicateSignal{Kind: "comment", Scope: fmt.Sprintf("TASK-%d", seq), Value: body, Count: count, TaskRef: fmt.Sprintf("TASK-%d", seq)})
	}
	return out, commentRows.Err()
}

func queryTopLabelsByChurn(ctx context.Context, db *sql.DB, projectID string, sinceAt int64, limit int) ([]labelChurnSignal, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT summary FROM activity_log
		WHERE project_id=? AND entity='label' AND created_at>=?`, projectID, sinceAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var summary string
		if err := rows.Scan(&summary); err != nil {
			return nil, err
		}
		label := quotedLabelName(summary)
		if label == "" {
			label = "unknown"
		}
		counts[label]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]labelChurnSignal, 0, len(counts))
	for label, count := range counts {
		out = append(out, labelChurnSignal{Label: label, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Label < out[j].Label
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func quotedLabelName(summary string) string {
	start := strings.IndexByte(summary, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(summary[start+1:], '"')
	if end < 0 {
		return ""
	}
	return summary[start+1 : start+1+end]
}

func queryCloseRatio(ctx context.Context, db *sql.DB, projectID string, sinceAt int64) ([]actorKindCount, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT actor_kind, COUNT(*)
		FROM activity_log
		WHERE project_id=? AND entity='task' AND action='status_changed'
		  AND summary LIKE '% to done' AND created_at>=?
		GROUP BY actor_kind ORDER BY actor_kind`, projectID, sinceAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []actorKindCount
	for rows.Next() {
		var row actorKindCount
		if err := rows.Scan(&row.ActorKind, &row.Count); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func actorLabel(kind, name string) string {
	if kind == "" && name == "" {
		return ""
	}
	if kind == "" {
		return name
	}
	if name == "" {
		return kind
	}
	return kind + ":" + name
}

func printActivityAnalysis(r *activityAnalysisReport) {
	fmt.Fprintf(os.Stdout, "Activity analysis for %s since %s\n\n", r.Project, r.Since)
	fmt.Fprintf(os.Stdout, "Tasks: created=%d completed=%d todo=%d doing=%d done=%d\n",
		r.Tasks.Created, r.Tasks.Completed, r.Tasks.Todo, r.Tasks.Doing, r.Tasks.Done)
	fmt.Fprintln(os.Stdout, "\nCycle time by type:")
	if len(r.CycleTimeByType) == 0 {
		fmt.Fprintln(os.Stdout, "- no completed tasks in window")
	} else {
		for _, row := range r.CycleTimeByType {
			fmt.Fprintf(os.Stdout, "- %s: %d completed, avg %s (min %s, max %s)\n", row.Type, row.Count, row.Average, row.Minimum, row.Maximum)
		}
	}
	fmt.Fprintln(os.Stdout, "\nStatus latency:")
	fmt.Fprintf(os.Stdout, "- todo -> doing: %d samples, avg %s\n", r.StatusLatency.TodoToDoingCount, r.StatusLatency.TodoToDoingAverage)
	fmt.Fprintf(os.Stdout, "- doing -> done: %d samples, avg %s\n", r.StatusLatency.DoingToDoneCount, r.StatusLatency.DoingToDoneAverage)
	fmt.Fprintln(os.Stdout, "\nWIP by actor:")
	printActorCounts(r.WIPByActor)
	fmt.Fprintln(os.Stdout, "\nQuality signals:")
	printTaskRefs("no completion evidence", r.NoCompletionEvidence)
	printTaskRefs("reopened tasks", r.ReopenedTasks)
	printTaskRefs("bug followups", r.BugFollowups)
	printTaskRefs("stale doing tasks", r.StaleDoingTasks)
	printDuplicates(r.Duplicates)
	printLabelChurn(r.TopLabelsByChurn)
	fmt.Fprintln(os.Stdout, "\nHuman vs AI closes:")
	if len(r.HumanVsAICloses) == 0 {
		fmt.Fprintln(os.Stdout, "- no closes in window")
		return
	}
	for _, row := range r.HumanVsAICloses {
		fmt.Fprintf(os.Stdout, "- %s: %d\n", row.ActorKind, row.Count)
	}
}

func printActorCounts(rows []analysisActorCount) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stdout, "- none")
		return
	}
	for _, row := range rows {
		fmt.Fprintf(os.Stdout, "- %s: %d\n", row.Actor, row.Count)
	}
}

func printTaskRefs(label string, refs []taskIssueRef) {
	if len(refs) == 0 {
		fmt.Fprintf(os.Stdout, "- %s: none\n", label)
		return
	}
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, ref.Ref)
	}
	fmt.Fprintf(os.Stdout, "- %s: %s\n", label, strings.Join(parts, ", "))
}

func printDuplicates(rows []duplicateSignal) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stdout, "- duplicate labels/plans/comments: none")
		return
	}
	fmt.Fprintln(os.Stdout, "- duplicate labels/plans/comments:")
	for _, row := range rows {
		scope := row.Scope
		if row.TaskRef != "" {
			scope = row.TaskRef
		}
		fmt.Fprintf(os.Stdout, "  - %s in %s: %q (%d)\n", row.Kind, scope, row.Value, row.Count)
	}
}

func printLabelChurn(rows []labelChurnSignal) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stdout, "- top labels by churn: none")
		return
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%s=%d", row.Label, row.Count))
	}
	fmt.Fprintf(os.Stdout, "- top labels by churn: %s\n", strings.Join(parts, ", "))
}
