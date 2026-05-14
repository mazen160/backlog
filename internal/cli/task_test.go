package cli

import (
	"context"
	"os"
	"testing"

	"github.com/mazen160/backlog/internal/migrate"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

// TestTaskProjectFieldAfterCreate is a regression guard for the bug fixed in
// commit e81be31 where TaskService.Create returned a task with an empty
// embedded Project struct. It verifies that after creating a project and a
// task inside it, the returned task has the correct Project.Alias populated.
func TestTaskProjectFieldAfterCreate(t *testing.T) {
	// Set up a temporary DB.
	f, err := os.CreateTemp("", "backlog-task-test-*.db")
	if err != nil {
		t.Fatalf("create temp db file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := repo.Open(f.Name())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := migrate.Run(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	actor := models.Actor{Kind: models.ActorKindHuman, Name: "testuser"}

	// Create a project with alias "smoke".
	projSvc := service.NewProjectService(db)
	_, err = projSvc.Create(ctx, models.CreateProjectInput{
		Alias: "smoke",
		Name:  "smoke",
		Actor: actor,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Resolve the project to get its ID.
	proj, err := projSvc.GetByAlias(ctx, "smoke")
	if err != nil {
		t.Fatalf("get project by alias: %v", err)
	}

	// Create a task in that project.
	planSvc := service.NewPlanService(db)
	labelSvc := service.NewLabelService(db)
	taskSvc := service.NewTaskService(db, planSvc, labelSvc)

	task, err := taskSvc.Create(ctx, models.CreateTaskInput{
		ProjectID: proj.ID,
		Title:     "test task",
		Actor:     actor,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Assert the Project field is populated with the correct alias.
	if task.Project == nil {
		t.Fatal("task.Project is nil; expected embedded project with alias \"smoke\"")
	}
	if task.Project.Alias != "smoke" {
		t.Fatalf("task.Project.Alias = %q; want \"smoke\"", task.Project.Alias)
	}
}
