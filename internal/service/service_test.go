package service_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mazen160/backlog/internal/migrate"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "backlog-test-*.db")
	require.NoError(t, err)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := repo.Open(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migrate.Run(db))
	return db
}

var testActor = models.Actor{Kind: models.ActorKindHuman, Name: "testuser"}

// ---- Project ----

func TestProjectCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	svc := service.NewProjectService(db)

	// Create
	p, err := svc.Create(ctx, models.CreateProjectInput{
		Alias: "myproject", Name: "My Project", Actor: testActor,
	})
	require.NoError(t, err)
	require.Equal(t, "myproject", p.Alias)

	// Get
	got, err := svc.GetByAlias(ctx, "myproject")
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ID)

	// List
	projects, err := svc.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)

	// Update
	newName := "Renamed"
	_, err = svc.Update(ctx, "myproject", models.UpdateProjectInput{Name: &newName}, testActor)
	require.NoError(t, err)
	got, _ = svc.GetByAlias(ctx, "myproject")
	require.Equal(t, "Renamed", got.Name)

	// Archive
	_, err = svc.Archive(ctx, "myproject", testActor)
	require.NoError(t, err)
	projects, _ = svc.List(ctx, false)
	require.Empty(t, projects)
	projects, _ = svc.List(ctx, true)
	require.Len(t, projects, 1)
	require.NotNil(t, projects[0].ArchivedAt)
}

func TestProjectAliasValidation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	svc := service.NewProjectService(db)

	cases := []struct {
		alias string
		ok    bool
	}{
		{"valid", true},
		{"valid-with-hyphens", true},
		{"abc123", true},
		{"UPPER", false},
		{"has space", false},
		{"", false},
		{"-starts-with-hyphen", false},
	}
	for _, c := range cases {
		_, err := svc.Create(ctx, models.CreateProjectInput{Alias: c.alias, Name: "N", Actor: testActor})
		if c.ok {
			require.NoError(t, err, "alias %q should be valid", c.alias)
		} else {
			require.Error(t, err, "alias %q should be invalid", c.alias)
		}
	}
}

// ---- Task ----

func TestTaskCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})

	// Create
	t1, err := taskSvc.Create(ctx, models.CreateTaskInput{
		ProjectID: p.ID,
		Title:     "Fix the bug",
		Type:      models.TaskTypeBug,
		Priority:  1,
		Actor:     testActor,
	})
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusTodo, t1.Status)

	// List
	tasks, total, err := taskSvc.List(ctx, models.TaskFilter{ProjectAlias: "proj"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, tasks, 1)

	// Get
	got, err := taskSvc.Get(ctx, t1.ID, false, false)
	require.NoError(t, err)
	require.Equal(t, "Fix the bug", got.Title)
	require.Equal(t, "proj", got.Project.Alias)

	// Move
	_, err = taskSvc.Move(ctx, t1.ID, models.TaskStatusDoing, testActor)
	require.NoError(t, err)
	got, _ = taskSvc.Get(ctx, t1.ID, false, false)
	require.Equal(t, models.TaskStatusDoing, got.Status)

	// Update
	newTitle := "Fix the critical bug"
	_, err = taskSvc.Update(ctx, t1.ID, models.UpdateTaskInput{Title: &newTitle}, testActor)
	require.NoError(t, err)
	got, _ = taskSvc.Get(ctx, t1.ID, false, false)
	require.Equal(t, "Fix the critical bug", got.Title)

	// Archive
	_, err = taskSvc.Archive(ctx, t1.ID, testActor)
	require.NoError(t, err)
	tasks, _, _ = taskSvc.List(ctx, models.TaskFilter{ProjectAlias: "proj"})
	require.Empty(t, tasks)

	// Delete
	t2, _ := taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "t2", Actor: testActor})
	require.NoError(t, taskSvc.Delete(ctx, t2.ID, testActor))
	_, err = taskSvc.Get(ctx, t2.ID, false, false)
	require.Error(t, err)
}

func TestTaskFilters(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})

	aiActor := models.Actor{Kind: models.ActorKindAI, Name: "claude-code"}
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "vuln", Type: models.TaskTypeVulnerability, Priority: 1, Actor: aiActor})
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "bug", Type: models.TaskTypeBug, Priority: 2, Actor: testActor})
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "done task", Status: models.TaskStatusDone, Actor: testActor})

	// Filter by type
	tasks, _, _ := taskSvc.List(ctx, models.TaskFilter{Type: models.TaskTypeVulnerability})
	require.Len(t, tasks, 1)
	require.Equal(t, "vuln", tasks[0].Title)

	// Filter by priority
	tasks, _, _ = taskSvc.List(ctx, models.TaskFilter{Priority: 1})
	require.Len(t, tasks, 1)

	// Filter by status
	tasks, _, _ = taskSvc.List(ctx, models.TaskFilter{Status: models.TaskStatusDone})
	require.Len(t, tasks, 1)

	// Filter by actor kind
	tasks, _, _ = taskSvc.List(ctx, models.TaskFilter{ActorKind: models.ActorKindAI})
	require.Len(t, tasks, 1)
	require.Equal(t, "vuln", tasks[0].Title)
}

func TestTaskFTS(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "JWT authentication bypass", Actor: testActor})
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "SQL injection in login", Actor: testActor})
	taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "Rate limiting missing", Actor: testActor})

	tasks, _, err := taskSvc.List(ctx, models.TaskFilter{Search: "JWT*"})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Contains(t, tasks[0].Title, "JWT")

	tasks, _, err = taskSvc.List(ctx, models.TaskFilter{Search: "injection"})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

// ---- Plan versioning ----

func TestPlanVersioning(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})
	task, _ := taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "task", Actor: testActor})

	// v1
	plan, err := planSvc.Create(ctx, models.CreatePlanInput{
		TaskID: task.ID,
		Title:  "Initial plan",
		Body:   "Step 1: do something",
		Actor:  testActor,
	})
	require.NoError(t, err)
	require.Equal(t, 1, plan.CurrentVersion)
	require.Equal(t, "Initial plan", plan.Version.Title)

	// v2
	plan, err = planSvc.Update(ctx, plan.ID, models.UpdatePlanInput{
		Title:      "Revised plan",
		Body:       "Step 1: do better",
		ChangeNote: "improved approach",
		Actor:      models.Actor{Kind: models.ActorKindAI, Name: "gpt-4"},
	})
	require.NoError(t, err)
	require.Equal(t, 2, plan.CurrentVersion)
	require.Equal(t, "Revised plan", plan.Version.Title)

	// v3
	plan, err = planSvc.Update(ctx, plan.ID, models.UpdatePlanInput{
		Title: "Final plan",
		Body:  "Step 1: ship it",
		Actor: testActor,
	})
	require.NoError(t, err)
	require.Equal(t, 3, plan.CurrentVersion)

	// Read v1
	v1, err := planSvc.Get(ctx, plan.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "Initial plan", v1.Version.Title)
	require.Equal(t, string(testActor.Kind), string(v1.Version.Actor.Kind))

	// Read v2
	v2, err := planSvc.Get(ctx, plan.ID, 2)
	require.NoError(t, err)
	require.Equal(t, "Revised plan", v2.Version.Title)
	require.Equal(t, "improved approach", v2.Version.ChangeNote)
	require.Equal(t, "gpt-4", v2.Version.Actor.Name)

	// History
	history, err := planSvc.History(ctx, plan.ID)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, 1, history[0].Version)
	require.Equal(t, 3, history[2].Version)

	// List for task
	plans, err := planSvc.ListForTask(ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, 3, plans[0].CurrentVersion) // shows current

	// Delete
	require.NoError(t, planSvc.Delete(ctx, plan.ID, testActor))
	plans, err = planSvc.ListForTask(ctx, task.ID)
	require.NoError(t, err)
	require.Empty(t, plans)
}

// ---- Comments ----

func TestComments(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)
	commentSvc := service.NewCommentService(db)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})
	task, _ := taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "task", Actor: testActor})

	c1, err := commentSvc.Create(ctx, models.CreateCommentInput{TaskID: task.ID, Body: "first", Actor: testActor})
	require.NoError(t, err)
	c2, err := commentSvc.Create(ctx, models.CreateCommentInput{TaskID: task.ID, Body: "second", Actor: testActor})
	require.NoError(t, err)

	comments, err := commentSvc.ListForTask(ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, comments, 2)

	require.NoError(t, commentSvc.Delete(ctx, c1.ID, testActor))
	comments, _ = commentSvc.ListForTask(ctx, task.ID)
	require.Len(t, comments, 1)
	require.Equal(t, c2.ID, comments[0].ID)
}

// ---- Labels ----

func TestLabels(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "proj", Name: "Proj", Actor: testActor})

	// Create label
	l, err := labelSvc.Create(ctx, models.CreateLabelInput{ProjectID: p.ID, Name: "security", Color: "#red"})
	require.NoError(t, err)
	require.Equal(t, "security", l.Name)

	// Idempotent create
	l2, err := labelSvc.Create(ctx, models.CreateLabelInput{ProjectID: p.ID, Name: "security"})
	require.NoError(t, err)
	require.Equal(t, l.ID, l2.ID)

	// Attach via task create
	task, _ := taskSvc.Create(ctx, models.CreateTaskInput{
		ProjectID: p.ID, Title: "labeled task",
		Labels: []string{"security", "newlabel"},
		Actor:  testActor,
	})
	got, _ := taskSvc.Get(ctx, task.ID, false, false)
	require.Len(t, got.Labels, 2)

	// Detach
	require.NoError(t, labelSvc.Detach(ctx, task.ID, p.ID, "security"))
	got, _ = taskSvc.Get(ctx, task.ID, false, false)
	require.Len(t, got.Labels, 1)
}

// ---- Import ----

func TestCrossDBImport(t *testing.T) {
	ctx := context.Background()

	// Source DB
	srcDB := openTestDB(t)
	srcProjSvc := service.NewProjectService(srcDB)
	srcPlanSvc := service.NewPlanService(srcDB)
	srcLabelSvc := service.NewLabelService(srcDB)
	srcTaskSvc := service.NewTaskService(srcDB, srcPlanSvc, srcLabelSvc)
	srcCommentSvc := service.NewCommentService(srcDB)

	p, _ := srcProjSvc.Create(ctx, models.CreateProjectInput{Alias: "src-proj", Name: "Source", Actor: testActor})
	task, _ := srcTaskSvc.Create(ctx, models.CreateTaskInput{
		ProjectID: p.ID, Title: "imported task",
		Type: models.TaskTypeVulnerability, Priority: 1,
		Labels: []string{"security"},
		Actor:  testActor,
	})
	plan, _ := srcPlanSvc.Create(ctx, models.CreatePlanInput{TaskID: task.ID, Title: "Plan v1", Body: "body", Actor: testActor})
	srcPlanSvc.Update(ctx, plan.ID, models.UpdatePlanInput{Title: "Plan v2", Body: "body2", Actor: testActor})
	srcCommentSvc.Create(ctx, models.CreateCommentInput{TaskID: task.ID, Body: "a comment", Actor: testActor})

	// Destination DB
	dstDB := openTestDB(t)
	importActor := models.Actor{Kind: models.ActorKindHuman, Name: "importer"}
	result, err := service.ImportFromDB(ctx, dstDB, srcDB, "", importActor, false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Tasks)
	require.Equal(t, 1, result.Plans)
	require.Equal(t, 1, result.Comments)

	// Verify in dst
	dstPlanSvc := service.NewPlanService(dstDB)
	dstLabelSvc := service.NewLabelService(dstDB)
	dstTaskSvc := service.NewTaskService(dstDB, dstPlanSvc, dstLabelSvc)
	tasks, total, err := dstTaskSvc.List(ctx, models.TaskFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, tasks, 1)
	require.Equal(t, "imported task", tasks[0].Title)

	// Plans with history preserved
	plans, err := dstPlanSvc.ListForTask(ctx, tasks[0].ID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, 2, plans[0].CurrentVersion)

	history, _ := dstPlanSvc.History(ctx, plans[0].ID)
	require.Len(t, history, 2)
}

func TestImportFindingsUsesDefaultProjectFallback(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, err := projSvc.Create(ctx, models.CreateProjectInput{Alias: "fallback", Name: "Fallback", Actor: testActor})
	require.NoError(t, err)

	findingsPath := t.TempDir() + "/findings.json"
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{
		"version": 1,
		"items": [
			{"title": "uses fallback project", "priority": "P2"}
		]
	}`), 0644))

	result, err := service.ImportFindings(ctx, db, findingsPath, "", "fallback", testActor, false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Tasks)

	tasks, total, err := taskSvc.List(ctx, models.TaskFilter{ProjectAlias: "fallback"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, tasks, 1)
	require.Equal(t, p.ID, tasks[0].ProjectID)
	require.Equal(t, "uses fallback project", tasks[0].Title)
}

func TestImportFindingsFileProjectWinsOverDefaultFallback(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	_, err := projSvc.Create(ctx, models.CreateProjectInput{Alias: "fallback", Name: "Fallback", Actor: testActor})
	require.NoError(t, err)
	p, err := projSvc.Create(ctx, models.CreateProjectInput{Alias: "file-proj", Name: "File Project", Actor: testActor})
	require.NoError(t, err)

	findingsPath := t.TempDir() + "/findings.json"
	require.NoError(t, os.WriteFile(findingsPath, []byte(`{
		"version": 1,
		"project": "file-proj",
		"items": [
			{"title": "uses file project", "priority": "P3"}
		]
	}`), 0644))

	result, err := service.ImportFindings(ctx, db, findingsPath, "", "fallback", testActor, false)
	require.NoError(t, err)
	require.Equal(t, 1, result.Tasks)

	tasks, total, err := taskSvc.List(ctx, models.TaskFilter{ProjectAlias: "file-proj"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, tasks, 1)
	require.Equal(t, p.ID, tasks[0].ProjectID)
	require.Equal(t, "uses file project", tasks[0].Title)
}

// ---- Manifest sync ----

func TestManifestSync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	dir := t.TempDir()
	// Write a backlog.json
	require.NoError(t, os.WriteFile(dir+"/backlog.json", []byte(`{
		"version": 1,
		"projects": [
			{"alias": "alpha", "name": "Alpha"},
			{"alias": "beta", "name": "Beta", "repo_path": "~/code/beta"}
		]
	}`), 0644))

	svc := service.NewManifestService(db)
	result, err := svc.Sync(ctx, dir, testActor)
	require.NoError(t, err)
	require.Len(t, result.Added, 2)
	require.Contains(t, result.Added, "alpha")
	require.Contains(t, result.Added, "beta")

	// Idempotent
	result, err = svc.Sync(ctx, dir, testActor)
	require.NoError(t, err)
	require.Empty(t, result.Added)
	require.Len(t, result.Skipped, 2)
}

func TestTaskSeqAndResolveRef(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	p, _ := projSvc.Create(ctx, models.CreateProjectInput{Alias: "seq", Name: "Seq", Actor: testActor})

	t1, err := taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "First", Actor: testActor})
	require.NoError(t, err)
	require.Equal(t, 1, t1.Seq)

	t2, err := taskSvc.Create(ctx, models.CreateTaskInput{ProjectID: p.ID, Title: "Second", Actor: testActor})
	require.NoError(t, err)
	require.Equal(t, 2, t2.Seq)

	// Resolve by TASK-N
	id, err := taskSvc.ResolveRef(ctx, "TASK-1")
	require.NoError(t, err)
	require.Equal(t, t1.ID, id)

	// Resolve by bare integer
	id, err = taskSvc.ResolveRef(ctx, "2")
	require.NoError(t, err)
	require.Equal(t, t2.ID, id)

	// Resolve by ULID passthrough
	id, err = taskSvc.ResolveRef(ctx, t1.ID)
	require.NoError(t, err)
	require.Equal(t, t1.ID, id)

	// Get by TASK-N ref
	got, err := taskSvc.Get(ctx, "TASK-2", false, false)
	require.NoError(t, err)
	require.Equal(t, "Second", got.Title)
	require.Equal(t, 2, got.Seq)

	// Non-existent TASK-N returns error
	_, err = taskSvc.ResolveRef(ctx, "TASK-999")
	require.Error(t, err)
}

// ---- Project delete cleanup (TASK-56, TASK-57) ----

func TestProjectDeleteRequiresArchived(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	svc := service.NewProjectService(db)

	p, err := svc.Create(ctx, models.CreateProjectInput{Alias: "active", Name: "Active", Actor: testActor})
	require.NoError(t, err)

	err = svc.Delete(ctx, p.Alias, testActor)
	require.Error(t, err, "deleting an active project should fail")
}

func TestProjectDeleteCleansAttachmentsAndActivity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projSvc := service.NewProjectService(db)
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)
	attachSvc := service.NewAttachmentService(db)

	// Create and archive a project with a task and two attachments.
	p, err := projSvc.Create(ctx, models.CreateProjectInput{Alias: "todel", Name: "To Delete", Actor: testActor})
	require.NoError(t, err)

	task, err := taskSvc.Create(ctx, models.CreateTaskInput{
		ProjectID: p.ID, Title: "Task", Type: models.TaskTypeBug, Priority: 3, Actor: testActor,
	})
	require.NoError(t, err)

	_, err = attachSvc.Add(ctx, "file.txt", []byte("data"), "task", task.ID, testActor)
	require.NoError(t, err)

	// Verify attachment exists.
	attachRepo := repo.NewAttachmentRepo(db)
	attachments, err := attachRepo.ListForLinked(ctx, "task", task.ID)
	require.NoError(t, err)
	require.Len(t, attachments, 1)

	// Archive then delete.
	_, err = projSvc.Archive(ctx, p.Alias, testActor)
	require.NoError(t, err)

	err = projSvc.Delete(ctx, p.Alias, testActor)
	require.NoError(t, err)

	// Attachment rows must be gone.
	attachments, err = attachRepo.ListForLinked(ctx, "task", task.ID)
	require.NoError(t, err)
	require.Empty(t, attachments)

	// Activity rows for that project must be gone.
	activityRepo := repo.NewActivityRepo(db)
	events, err := activityRepo.List(ctx, p.ID, "", "", "", 100, 0)
	require.NoError(t, err)
	require.Empty(t, events)
}
