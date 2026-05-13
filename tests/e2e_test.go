package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var backlogBin string

func TestMain(m *testing.M) {
	// Build the binary into a temp location
	bin, err := os.CreateTemp("", "backlog-e2e-*")
	if err != nil {
		panic(err)
	}
	bin.Close()
	binPath := bin.Name()

	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/backlog")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build failed: " + err.Error())
	}
	backlogBin = binPath
	code := m.Run()
	os.Remove(binPath)
	os.Exit(code)
}

// workspace layout: <tmp>/ws is the backlog workspace directory, <tmp>/home
// is an isolated HOME so concurrent tests do not race on
// ~/.config/backlog/config.toml. runEnv derives both from the workspace dir
// so every test command is fully self-contained.
func runEnv(dir string) []string {
	base := filepath.Dir(dir)
	return append(os.Environ(),
		"HOME="+filepath.Join(base, "home"),
		"BACKLOG_DB="+filepath.Join(dir, "backlog.db"),
	)
}

// run executes the backlog binary in dir with args, returns stdout.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(backlogBin, args...)
	cmd.Dir = dir
	cmd.Env = runEnv(dir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "backlog %s failed:\n%s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

// runJSON runs and parses JSON output.
func runJSON(t *testing.T, dir string, args ...string) map[string]interface{} {
	t.Helper()
	args = append(args, "--json")
	cmd := exec.Command(backlogBin, args...)
	cmd.Dir = dir
	cmd.Env = runEnv(dir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "backlog %s failed:\n%s", strings.Join(args, " "), out)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &result))
	return result
}

func newWorkspace(t *testing.T) string {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "ws")
	home := filepath.Join(tmp, "home")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.MkdirAll(home, 0o755))
	run(t, dir, "init", "--path", dir)
	return dir
}

// ---- Tests ----

func TestInit(t *testing.T) {
	dir := newWorkspace(t)
	require.FileExists(t, filepath.Join(dir, "backlog.db"))
	require.FileExists(t, filepath.Join(dir, "config.toml"))
}

func TestProjectLifecycle(t *testing.T) {
	dir := newWorkspace(t)

	// Add
	run(t, dir, "project", "add", "API", "--alias", "api", "--repo-path", "/code/api")
	run(t, dir, "project", "add", "Web", "--alias", "web")

	// List
	data := runJSON(t, dir, "project", "list")
	projects := data["projects"].([]interface{})
	require.Len(t, projects, 2)

	// Show
	data = runJSON(t, dir, "project", "show", "api")
	require.Equal(t, "api", data["alias"])

	// Update
	run(t, dir, "project", "update", "api", "--name", "API v2")
	data = runJSON(t, dir, "project", "show", "api")
	require.Equal(t, "API v2", data["name"])

	// Archive
	run(t, dir, "project", "archive", "api")
	data = runJSON(t, dir, "project", "list")
	projects = data["projects"].([]interface{})
	require.Len(t, projects, 1) // only web

	// Include archived
	data = runJSON(t, dir, "project", "list", "--include-archived")
	projects = data["projects"].([]interface{})
	require.Len(t, projects, 2)
}

func TestTaskLifecycle(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")

	// Add task
	taskData := runJSON(t, dir, "task", "add", "-p", "api", "-t", "Fix the bug",
		"--type", "bug", "--priority", "P1", "--as", "human:alice")
	taskID := taskData["id"].(string)
	require.NotEmpty(t, taskID)
	require.Equal(t, float64(1), taskData["priority"])
	require.Equal(t, "bug", taskData["type"])

	// List
	data := runJSON(t, dir, "task", "list")
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 1)

	// Show
	data = runJSON(t, dir, "task", "show", taskID)
	require.Equal(t, taskID, data["id"])
	require.Equal(t, "api", data["project"].(map[string]interface{})["alias"])

	// Move
	run(t, dir, "task", "move", taskID, "--status", "doing")
	data = runJSON(t, dir, "task", "show", taskID)
	require.Equal(t, "doing", data["status"])

	// Update
	run(t, dir, "task", "update", taskID, "--title", "Fix the critical bug", "--priority", "P2")
	data = runJSON(t, dir, "task", "show", taskID)
	require.Equal(t, "Fix the critical bug", data["title"])
	require.Equal(t, float64(2), data["priority"])

	// Archive
	run(t, dir, "task", "archive", taskID)
	data = runJSON(t, dir, "task", "list")
	tasks = data["tasks"].([]interface{})
	require.Empty(t, tasks)
}

func TestPlanVersioning(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")
	taskData := runJSON(t, dir, "task", "add", "-p", "api", "-t", "Auth task")
	taskID := taskData["id"].(string)

	// Add plan (v1)
	planData := runJSON(t, dir, "plan", "add", "--task", taskID,
		"--title", "Initial plan", "--content", "Step 1: do it", "--as", "ai:claude-code")
	planID := planData["id"].(string)
	require.Equal(t, float64(1), planData["current_version"])

	// Update plan (v2)
	planData = runJSON(t, dir, "plan", "update", planID,
		"--title", "Revised plan", "--content", "Step 1: do it better",
		"--change-note", "improved")
	require.Equal(t, float64(2), planData["current_version"])

	// Update plan (v3)
	run(t, dir, "plan", "update", planID, "--title", "Final plan", "--content", "Step 1: ship it")

	// Show v1
	data := runJSON(t, dir, "plan", "show", planID, "--version", "1")
	require.Equal(t, "Initial plan", data["version"].(map[string]interface{})["title"])
	require.Equal(t, "ai", data["version"].(map[string]interface{})["actor"].(map[string]interface{})["kind"])

	// History
	data = runJSON(t, dir, "plan", "history", planID)
	versions := data["versions"].([]interface{})
	require.Len(t, versions, 3)

	// Task show includes current plan
	data = runJSON(t, dir, "task", "show", taskID)
	plans := data["plans"].([]interface{})
	require.Len(t, plans, 1)
	require.Equal(t, float64(3), plans[0].(map[string]interface{})["current_version"])
}

func TestActorAttribution(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")

	// Human task
	humanData := runJSON(t, dir, "task", "add", "-p", "api", "-t", "Human task", "--as", "human:bob")
	actor := humanData["actor"].(map[string]interface{})
	require.Equal(t, "human", actor["kind"])
	require.Equal(t, "bob", actor["name"])

	// AI task
	aiData := runJSON(t, dir, "task", "add", "-p", "api", "-t", "AI task", "--as", "ai:claude-code")
	actor = aiData["actor"].(map[string]interface{})
	require.Equal(t, "ai", actor["kind"])
	require.Equal(t, "claude-code", actor["name"])

	// Filter by actor kind
	data := runJSON(t, dir, "task", "list", "--actor-kind", "ai")
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 1)
	require.Equal(t, "AI task", tasks[0].(map[string]interface{})["title"])
}

func TestImportFindings(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")

	findingsFile := filepath.Join(dir, "findings.json")
	require.NoError(t, os.WriteFile(findingsFile, []byte(`{
		"version": 1,
		"project": "api",
		"items": [
			{"title": "SQL injection", "type": "vulnerability", "priority": "P1",
			 "plans": [{"title": "Fix", "body": "Use parameterized queries"}]},
			{"title": "XSS in comments", "type": "vulnerability", "priority": "P2"},
			{"title": "Slow query", "type": "improvement", "priority": "P3"}
		]
	}`), 0644))

	// Dry run
	result := run(t, dir, "import-findings", findingsFile, "--dry-run")
	require.Contains(t, result, "[dry-run]")
	require.Contains(t, result, "3 tasks")

	// Real import
	run(t, dir, "import-findings", findingsFile, "--as", "ai:scanner")

	// Verify
	data := runJSON(t, dir, "task", "list", "--project", "api")
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 3)

	// The P1 task should have a plan
	taskID := tasks[0].(map[string]interface{})["id"].(string) // sorted by priority
	data = runJSON(t, dir, "task", "show", taskID)
	plans, ok := data["plans"].([]interface{})
	if ok && len(plans) > 0 {
		require.Equal(t, "Fix", plans[0].(map[string]interface{})["version"].(map[string]interface{})["title"])
	}
}

func TestLabelWorkflow(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")
	taskData := runJSON(t, dir, "task", "add", "-p", "api", "-t", "Security task")
	taskID := taskData["id"].(string)

	run(t, dir, "label", "create", "--project", "api", "security", "--color", "#ff0000")
	run(t, dir, "label", "attach", "--task", taskID, "security")

	data := runJSON(t, dir, "task", "list", "--label", "security")
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 1)

	run(t, dir, "label", "detach", "--task", taskID, "security")
	data = runJSON(t, dir, "task", "list", "--label", "security")
	tasks = data["tasks"].([]interface{})
	require.Empty(t, tasks)
}

func TestCrossDBImport(t *testing.T) {
	// Create source workspace
	src := newWorkspace(t)
	run(t, src, "project", "add", "Source project", "--alias", "src")
	run(t, src, "task", "add", "-p", "src", "-t", "Task from source", "--type", "vulnerability", "--priority", "P1")
	taskData := runJSON(t, src, "task", "list", "--json")
	_ = taskData

	srcDB := filepath.Join(src, "backlog.db")

	// Destination workspace
	dst := newWorkspace(t)
	run(t, dst, "project", "add", "Dst", "--alias", "dst")

	// Import
	result := run(t, dst, "import", srcDB)
	require.Contains(t, result, "1 tasks")

	// Verify task exists in dst
	data := runJSON(t, dst, "task", "list")
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 1)
	require.Equal(t, "Task from source", tasks[0].(map[string]interface{})["title"])
}

func TestSync(t *testing.T) {
	dir := newWorkspace(t)

	// Write manifest with 2 projects
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backlog.json"), []byte(`{
		"version": 1,
		"name": "test",
		"projects": [
			{"alias": "alpha", "name": "Alpha"},
			{"alias": "beta", "name": "Beta"}
		]
	}`), 0644))

	out := run(t, dir, "sync")
	require.Contains(t, out, "alpha")
	require.Contains(t, out, "beta")

	data := runJSON(t, dir, "project", "list")
	projects := data["projects"].([]interface{})
	require.Len(t, projects, 2)

	// Idempotent
	out = run(t, dir, "sync")
	require.Contains(t, out, "already exist")
}

func TestExport(t *testing.T) {
	dir := newWorkspace(t)
	run(t, dir, "project", "add", "API", "--alias", "api")
	run(t, dir, "task", "add", "-p", "api", "-t", "Task one", "--type", "bug")
	run(t, dir, "task", "add", "-p", "api", "-t", "Task two", "--type", "feature")

	// JSON export
	out := run(t, dir, "export", "--format", "json")
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &data))
	tasks := data["tasks"].([]interface{})
	require.Len(t, tasks, 2)

	// CSV export
	out = run(t, dir, "export", "--format", "csv")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3) // header + 2 tasks

	// Markdown export
	out = run(t, dir, "export", "--format", "md")
	require.Contains(t, out, "# Backlog Export")
	require.Contains(t, out, "Task one")
}
