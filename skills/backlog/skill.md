---
description: Interact with the Backlog CLI — create and manage tasks, plans, comments, labels, projects, memory, docs, and attachments in a local SQLite workspace
---

You have access to the `backlog` CLI. Use it to manage tasks, plans, comments, labels, projects, memory entries, docs, and attachments. Always pass `--as ai:<your-model-name>` so writes are attributed to you. Always pass `--json` when you need to parse output.

## Session startup

When starting a work session on a backlog project, load context before anything else:

```sh
# 1. Resolve the active project
backlog project list --json --profile default

# 2. Load persisted memory (decisions, architecture, open-work summaries)
backlog memory list --project <alias> --json --profile default

# 3. Load docs (full body of each)
backlog doc list --project <alias> --json --profile default
# For each doc ID returned:
backlog doc show <doc-id> --json --profile default

# 4. Load open tasks for situational awareness
backlog task list --project <alias> --status todo --json --profile default
backlog task list --project <alias> --status doing --json --profile default
```

Surface memory entries and doc bodies as context before responding — this prevents re-deriving decisions already recorded.

If memory entries are empty → suggest running `/backlog-memory-learn <alias>` first, then `/backlog-memory-store <alias>` to persist summaries.

## Core workflow

The standard agentic loop for working through a backlog:

### 1. Pick a task

```sh
# List by priority (highest first)
backlog task list --project <alias> --status todo --json --profile default
```

Choose the highest-priority task that is actionable. Prefer P1 > P2 > P3.

### 2. Claim it

```sh
backlog task move TASK-N --status doing --as "ai:<model>" --profile default
```

### 3. Attach a plan (for non-trivial tasks)

```sh
backlog plan add --task TASK-N \
  --title "Implementation plan" \
  --content "## Steps\n1. ...\n2. ...\n\n## Testing\n- ..." \
  --as "ai:<model>" --profile default
```

### 4. Do the work

Implement, fix, or research — whatever the task requires.

### 5. Record outcomes

```sh
# Add a comment with what was done / any decisions made
backlog comment add "Fixed by changing X in file Y. Verified with Z." \
  --task TASK-N --as "ai:<model>" --profile default

# If a decision was made that future sessions should know:
backlog memory add "Decided to use X because Y" \
  --project <alias> --tag "decision" --as "ai:<model>" --profile default
```

### 6. Close it

```sh
backlog task move TASK-N --status done --as "ai:<model>" --profile default
```

### 7. Repeat or stop

Pick the next task or surface a summary of what was completed.

## Task triage workflow

When asked to triage or bulk-create tasks from findings (scan output, review notes, etc.):

```sh
# 1. Write a findings JSON file
# 2. Dry-run first
backlog import-findings findings.json --dry-run --profile default
# 3. Import
backlog import-findings findings.json --as "ai:<model>" --profile default
# 4. Confirm
backlog task list --project <alias> --status todo --json --profile default
```

Findings file format:
```json
{
  "version": 1,
  "project": "<alias>",
  "items": [
    { "title": "...", "type": "bug", "priority": "P2", "source": "review" }
  ]
}
```

## Memory workflow

- **Learn** (read into context): `/backlog-memory-learn <alias>`
- **Store** (persist synthesized summaries): `/backlog-memory-store <alias>`
- Run store after significant work to refresh the `open-work` and `done-work` entries.

## Conventions

- Always `--profile default` unless the user specifies otherwise.
- Always `--as ai:<your-model-name>` on writes.
- Always `--json` when parsing output.
- Use `TASK-N` format in messages to the user.
- Never hardcode actor names — use `ai:<your-model-name>` dynamically.

## Core concepts

| Concept | Description |
|---|---|
| Workspace | Directory containing `backlog.db` + `config.toml`. Resolved via `--db`, `$BACKLOG_DB`, `--profile`, or the default profile. There is **no** cwd walk-up. |
| Profile | Named pointer to a workspace, registered in `~/.config/backlog/config.toml`. By default workspaces live at `~/.config/backlog/<profile-name>/`. |
| Project | Named group of tasks inside a workspace, identified by a short `alias` (e.g. `api`, `web`). |
| Task | Unit of work. Has type, status, priority, actor. Identified by `TASK-N`, bare `N`, or full ULID. |
| Plan | Versioned markdown document attached to a task. Every edit creates a new immutable version. |
| Doc | Versioned markdown document attached to a **project** (not a task). Same versioning model as plans. |
| Memory | Free-form note attached to a project. Body + comma-separated tags. Newest-first list, tag filter. |
| Attachment | Binary file attached to a task or a doc, stored in the SQLite DB. |
| Comment | Actor-attributed note on a task. |
| Label | Per-project tag attachable to tasks. |
| Actor | `kind:name` — `kind` is `human` or `ai`. Example: `ai:claude-code`. |

## ID formats — all equivalent for tasks

```
TASK-5          # canonical human-readable ref
5               # bare integer
01KR4JA4754H... # full ULID (returned in JSON)
```

Use the `TASK-N` format in messages to users. Use the ULID from JSON output when chaining commands. Plan IDs, doc IDs, attachment IDs, and memory IDs are always full or short ULIDs — there is no `PLAN-N` form.

## Global flags (apply to every command)

```
--json          output machine-readable JSON (always use when parsing)
--as kind:name  actor attribution, e.g. --as ai:claude-code
--db <path>     explicit path to backlog.db
--profile <n>   named workspace profile
--quiet         suppress success messages
```

## Environment

```
BACKLOG_DB=/path/to/backlog.db   # overrides profile resolution; use in MCP config
```

## DB resolution order

1. `--db <path>` flag
2. `$BACKLOG_DB` env var
3. `--profile <name>` flag → `~/.config/backlog/<name>/backlog.db`
4. Default profile from `~/.config/backlog/config.toml`
5. Error: "no backlog workspace found — run `backlog init` to create one"

---

## Init (workspace setup)

`backlog init` creates a workspace and registers it as a profile. By default it lives at `~/.config/backlog/<profile>/`; use `--path` to put it inside a project directory or a separate git repo.

```sh
# Default profile, default location (~/.config/backlog/default/)
backlog init

# Named profile, default location (~/.config/backlog/work/)
backlog init --profile work

# Named profile at a custom path
backlog init --profile myapp --path ~/code/myapp/.backlog

# Set as the active profile after creation
backlog init --profile work --set-default

# Wipe and recreate an existing workspace
backlog init --profile work --reset

# Pre-fill workspace defaults
backlog init --profile work --actor human:mazin --priority 2 --type bug --status doing
```

Flags:
- `--profile <name>` — profile name (default: `default`)
- `--path <dir>` — workspace directory (default: `~/.config/backlog/<profile>/`)
- `--set-default` — make this the active profile
- `--reset` — wipe and reinitialize an existing workspace
- `--actor`, `--priority`, `--status`, `--type` — defaults written into workspace `config.toml`

If no default profile exists yet, the new workspace becomes the default automatically.

---

## Profiles

```sh
# Register an existing workspace dir as a profile
backlog profile add work --path ~/projects/work-backlog

# Show all profiles (* marks the active one)
backlog profile list

# Show a single profile's resolved path
backlog profile show work

# Show the active profile
backlog profile current

# Switch the active profile
backlog profile use personal       # alias for set-default
backlog profile set-default personal

# Remove from registry (does not delete the workspace files)
backlog profile remove old

# Run a command against a non-default profile
backlog task list --profile work --json
```

---

## Projects

```sh
# Create
backlog project add "API Service" --alias api
backlog project add "Web Frontend" --alias web --repo-path /code/web

# List
backlog project list --json
backlog project list --include-archived --json

# Show
backlog project show api --json

# Update
backlog project update api --name "API v2" --description "REST backend"

# Archive (hidden from list; tasks remain)
backlog project archive api

# Delete (hard — removes all tasks, plans, comments, labels)
backlog project delete api
```

JSON shape of a project:
```json
{ "id": "...", "alias": "api", "name": "API Service", "description": "", "repo_path": "", "created_at": 0, "updated_at": 0 }
```

---

## Tasks

### Create

```sh
backlog task add \
  --project api \
  --title "Fix SQL injection in /search" \
  --description "Parameterize the query at internal/handlers/search.go:84" \
  --type vulnerability \
  --priority P1 \
  --source "semgrep" \
  --external-ref "SEMGREP-042" \
  --label security \
  --as ai:claude-code
```

Flags:
- `-p / --project` alias (required)
- `-t / --title` (required)
- `-d / --description` markdown body
- `--type` task · bug · issue · improvement · feature · vulnerability · chore · spike
- `--priority` P1–P5 or 1–5 (P1 = highest, P3 = default)
- `--status` todo · doing · done (default: todo)
- `--assignee` name
- `--label` repeatable: `--label auth --label crypto`
- `--source` origin tool/review name
- `--external-ref` URL or ticket ID
- `--from-file <file>` — `.json` is parsed as a full task payload, anything else is loaded as the description
- `--due-date` YYYY-MM-DD or RFC3339

### List

```sh
backlog task list --json
backlog task list --project api --json
backlog task list --status todo --json
backlog task list --type vulnerability --priority P1 --json
backlog task list --label security --json
backlog task list --actor-kind ai --json
backlog task list --actor-name claude-code --json
backlog task list --source semgrep --json
backlog task list --search "injection" --json    # FTS5
backlog task list --search "sql*" --json         # prefix
backlog task list --include-archived --json
backlog task list --limit 20 --offset 0 --json
backlog task list --sort seq --json           # sort by TASK-N order
backlog task list --sort created --json       # newest first
backlog task list --sort updated --json
backlog task list --sort title --json
```

JSON response shape (CLI):
```json
{
  "tasks": [
    {
      "id": "01KR...",
      "seq": 1,
      "project_id": "...",
      "project": { "alias": "api", "name": "API Service" },
      "title": "Fix SQL injection",
      "type": "vulnerability",
      "status": "todo",
      "priority": 1,
      "actor": { "kind": "ai", "name": "claude-code" },
      "source": "semgrep",
      "external_ref": "SEMGREP-042",
      "labels": [{ "name": "security", "color": "" }],
      "created_at": 1746724800000000000,
      "updated_at": 1746724800000000000
    }
  ],
  "page": { "total": 12, "count": 12 }
}
```

### Show

```sh
backlog task show TASK-1 --json
backlog task show TASK-1 --with-plans=false --with-comments=false --json   # disable defaults
```

`--with-plans` and `--with-comments` are both `true` by default.

### Update

Only provided flags are changed:
```sh
backlog task update TASK-1 --title "New title" --priority P2 --as ai:claude-code
backlog task update TASK-1 --assignee alice --source pentest-report
backlog task update TASK-1 --due-date 2026-06-01
```

### Move status

```sh
backlog task move TASK-1 --status doing --as ai:claude-code
backlog task move TASK-1 --status done  --as ai:claude-code
backlog task move TASK-1 --status todo  --as ai:claude-code
```

### Archive / Delete

```sh
backlog task archive TASK-1   # soft — hidden from list, recoverable via --include-archived
backlog task delete  TASK-1   # hard — permanent
```

---

## Plans

Plans are versioned. Every `plan update` creates a new immutable version. The current version is always returned by default.

### Add plan (creates v1)

```sh
backlog plan add \
  --task TASK-1 \
  --title "Remediation plan" \
  --content "## Steps\n1. Replace string interpolation with parameterized queries\n2. Add regression test" \
  --as ai:claude-code --json
```

To load body from a file:
```sh
backlog plan add --task TASK-1 --title "Plan" --from-file plan.md --as ai:claude-code
```

JSON response:
```json
{
  "id": "01KR...",
  "task_id": "...",
  "current_version": 1,
  "version": {
    "id": "...", "version": 1,
    "title": "Remediation plan", "body": "## Steps\n...",
    "actor": { "kind": "ai", "name": "claude-code" },
    "created_at": 0
  }
}
```

### Update plan (creates v2, v3, …)

```sh
backlog plan update <plan-id> \
  --title "Revised plan" \
  --content "## Steps\n1. Parameterize queries\n2. Rotate credentials\n3. Add regression test" \
  --change-note "added credential rotation" \
  --as human:alice --json
```

`--title` is required on update; `--from-file` works the same as on `add`.

### Show, history, list, delete

```sh
backlog plan show <plan-id> --json              # current version
backlog plan show <plan-id> --version 1 --json
backlog plan history <plan-id> --json
# { "versions": [ { "version": 1, "title": "...", "actor": {...}, "change_note": "", "created_at": 0 }, ... ] }

backlog plan list --task TASK-1 --json
backlog plan delete <plan-id>
```

---

## Comments

```sh
backlog comment add "Verified fix in PR #142." --task TASK-1 --as ai:claude-code
backlog comment list --task TASK-1 --json
backlog comment delete <comment-id>
```

The body is a positional argument; `--task` is a required flag.

---

## Labels

```sh
# Create (per-project)
backlog label create "security" --project api --color "#ff0000"

# List
backlog label list --project api

# Attach / detach (use ULID, not TASK-N — task ref is resolved internally)
backlog label attach security --task TASK-1
backlog label detach security --task TASK-1
```

---

## Memory (project-scoped notes)

Free-form text + optional comma-separated tags. Use this for decisions, context, design notes that aren't a task.

```sh
# Add
backlog memory add "Decided to use SQLite — single file, no server" \
  --project api --tag "decision,arch" --as ai:claude-code

# Append text to an existing entry (newline-joined)
backlog memory append <memory-id> "Confirmed at 2026-05-09 review meeting"

# List
backlog memory list --project api --json
backlog memory list --project api --tag arch --json   # filter by tag

# Delete
backlog memory delete <memory-id>
```

JSON entry shape:
```json
{
  "id": "01KR...",
  "project_id": "...",
  "body": "Decided to use SQLite — single file, no server",
  "tags": "decision,arch",
  "actor": { "kind": "ai", "name": "claude-code" },
  "created_at": 1746724800000000000
}
```

---

## Docs (project-scoped versioned documents)

Like plans, but attached to a project and used for longer-form documentation (architecture overviews, runbooks, design docs).

```sh
# Add (creates v1)
backlog doc add --project api --title "Architecture Overview" \
  --content "## Stack\n- Go 1.25\n- SQLite" --as ai:claude-code --json

backlog doc add --project api --title "Runbook" --from-file runbook.md

# List
backlog doc list --project api --json

# Show current version
backlog doc show <doc-id> --json

# Update (creates a new version)
backlog doc update <doc-id> --title "Architecture v2" \
  --content "..." --change-note "added queue layer" --as ai:claude-code

# Append (creates a new version with old body + new body joined)
backlog doc append <doc-id> --content "## New section\n..." \
  --change-note "extended troubleshooting"

# History
backlog doc history <doc-id> --json

# Delete
backlog doc delete <doc-id>
```

`--from-file` is supported on both `add`, `update`, and `append`.

---

## Attachments

Binary files attached to a task or a doc, stored inside the SQLite DB. The `attachment` command also has the alias `attach`.

```sh
# Attach a file to a task
backlog attachment add ./report.pdf --task TASK-1 --as ai:claude-code

# Attach to a doc
backlog attachment add ./diagram.png --doc <doc-id>

# List for a task or doc
backlog attachment list --task TASK-1 --json
backlog attachment list --doc <doc-id>

# Fetch (defaults to the original filename)
backlog attachment fetch <attachment-id>
backlog attachment fetch <attachment-id> --out /tmp/report.pdf
backlog attachment fetch <attachment-id> --out -          # stream to stdout

# Delete
backlog attachment delete <attachment-id>
```

---

## Import findings (bulk task creation)

Write a findings JSON file, then import it. This is the primary agentic intake path.

### File format

```json
{
  "version": 1,
  "project": "api",
  "items": [
    {
      "title": "SQL injection in /search",
      "type": "vulnerability",
      "priority": "P1",
      "source": "semgrep",
      "external_ref": "SEMGREP-001",
      "plans": [
        { "title": "Remediation", "body": "Use parameterized queries." }
      ]
    },
    {
      "title": "Outdated dependency: lodash 4.17.20",
      "type": "vulnerability",
      "priority": "P3",
      "source": "dependency-scan"
    }
  ]
}
```

`priority` accepts `"P1"`–`"P5"` or integers `1`–`5`.

### Run

```sh
backlog import-findings findings.json --dry-run            # count only, no writes
backlog import-findings findings.json --as ai:scanner
backlog import-findings findings.json --project web --as ai:scanner  # override project
```

---

## Cross-workspace import

```sh
backlog import /path/to/other/backlog.db --dry-run
backlog import /path/to/other/backlog.db --as ai:importer
backlog import /path/to/other/backlog.db --project api     # single project only
```

---

## Export

```sh
backlog export --format json   # full task objects → stdout
backlog export --format csv
backlog export --format md     # human-readable Markdown report
backlog export --format json --project api --out tasks.json
```

---

## Sync (manifest → DB)

```sh
backlog sync    # reads backlog.json from the workspace, creates missing projects, idempotent
```

---

## Activity log

```sh
backlog activity               # show last 50 events
backlog activity --limit 20    # limit results
backlog activity --json        # machine-readable
```

---

## Web UI

```sh
backlog web                    # serves on http://localhost:8080, opens browser
backlog web --port 3000        # custom port
backlog web --no-browser       # don't auto-open
```

The web UI provides task list/detail, doc browser, memory browser, and project list. It reads/writes the same workspace as the CLI.

---

## Schema (JSON Schema for payloads)

```sh
backlog schema --json     # JSON Schema for task, findings_file, manifest, actor
```

Useful when an agent needs a machine-readable contract for `--from-file` payloads.

---

## Shell completion

```sh
backlog completion bash   > /etc/bash_completion.d/backlog
backlog completion zsh    > ~/.zsh/completions/_backlog
backlog completion fish   > ~/.config/fish/completions/backlog.fish
backlog completion powershell
```

---

## Doctor / maintenance

```sh
backlog doctor check          # PRAGMA integrity_check
backlog doctor backup --to /safe/backlog.backup.db   # atomic VACUUM INTO
```

If `--to` is omitted, backup is written to `<workspace>/backlog.backup.db`.

---

## MCP server

When running as an MCP server, the same operations are available as tools. Start the server:
```sh
backlog mcp serve --as ai:claude-code --db /path/to/backlog.db
```

### Available MCP tools

| Tool | Required | Optional |
|---|---|---|
| `project_list` | — | — |
| `task_create` | `project`, `title` | `description`, `type`, `status`, `priority`, `source`, `external_ref`, `due_date` |
| `task_list` | — | `project`, `status`, `type`, `priority`, `search` |
| `task_show` | `id` | — |
| `task_update` | `id` | `title`, `description`, `status`, `priority`, `due_date` |
| `task_move` | `id`, `status` | — |
| `plan_add` | `task_id`, `title`, `body` | `source` |
| `plan_update` | `plan_id`, `title`, `body` | `change_note` |
| `plan_history` | `plan_id` | — |
| `comment_add` | `task_id`, `body` | — |
| `memory_add` | `project`, `body` | `tags` |
| `memory_list` | `project` | `tag` |
| `doc_add` | `project`, `title`, `body` | — |
| `doc_list` | `project` | — |
| `doc_show` | `id` | — |
| `doc_update` | `id`, `body` | `title`, `change_note` |

Note: `id` / `task_id` accept any of the three task ref forms (TASK-N, bare integer, ULID). `plan_id` and doc `id` are full ULIDs returned by the matching list/create response.

---

## Common agent workflows

### Triage findings from a scan

```sh
# 1. Write findings.json from scan output
# 2. Dry run
backlog import-findings findings.json --dry-run
# 3. Import
backlog import-findings findings.json --as ai:scanner
# 4. Confirm
backlog task list --actor-name scanner --json
```

### Pick up a task and attach a plan

```sh
backlog task move TASK-5 --status doing --as ai:claude-code
backlog plan add --task TASK-5 --title "Implementation plan" \
  --content "..." --as ai:claude-code --json
# → capture plan ID from response
```

### Revise a plan after human feedback

```sh
backlog plan update <plan-id> \
  --title "Revised plan" \
  --content "..." \
  --change-note "incorporated Alice's review" \
  --as ai:claude-code --json
```

### Capture a design decision as memory

```sh
backlog memory add "Chose Cobra over urfave/cli — better completion + posix flag handling" \
  --project api --tag "decision,deps" --as ai:claude-code
```

### Maintain a versioned runbook

```sh
backlog doc add --project api --title "Incident Runbook" \
  --from-file runbook.md --as ai:claude-code
backlog doc append <doc-id> --content "## 2026-05 outage RCA\n..." \
  --change-note "post-mortem section" --as ai:claude-code
backlog doc history <doc-id> --json
```

### Scripting with JSON output

```sh
# Get all open P1 vulnerabilities and iterate
backlog task list --type vulnerability --priority P1 --status todo --json \
  | jq -r '.tasks[].id'

# Create a task and capture its ID
TASK_ID=$(backlog task add -p api -t "Fix XSS" --type vulnerability --priority P2 \
  --as ai:claude-code --json | jq -r '.id')

# Add a plan using the captured ID
backlog plan add --task "$TASK_ID" --title "Plan" --content "..." --as ai:claude-code
```

---

## Enum reference

**type:** `task` · `bug` · `issue` · `improvement` · `feature` · `vulnerability` · `chore` · `spike`

**status:** `todo` · `doing` · `done`

**priority:** `1`/`P1` (critical) · `2`/`P2` (high) · `3`/`P3` (normal, default) · `4`/`P4` (low) · `5`/`P5` (backlog)

**actor.kind:** `human` · `ai`

**sort (task list):** `priority` (default) · `created` · `updated` · `seq` · `title`
