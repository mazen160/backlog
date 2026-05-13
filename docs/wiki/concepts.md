# Core Concepts

## Workspace

A directory that contains a `backlog.db` SQLite file and an optional `config.toml`. All data for a set of projects — tasks, plans, docs, comments, memory, labels, attachments, activity — lives in that single `backlog.db` file.

Workspaces are resolved via the DB resolution chain (see [architecture.md](architecture.md)): explicit `--db` path, `$BACKLOG_DB` env var, named `--profile`, or the default profile. There is no automatic search of the current directory or its parents.

A workspace is created with `backlog init`.

## Profile

A named pointer to a workspace directory, registered in `~/.config/backlog/config.toml`. The registry maps profile names to directory paths. For example, a profile named `work` might point to `~/projects/work/.backlog`, and the workspace file would be at `~/projects/work/.backlog/backlog.db`.

Default workspaces are placed at `~/.backlog/<profile-name>/`. The active (default) profile is stored under `default_profile` in the global config and used when no `--profile` or `--db` flag is given.

Profile operations: `backlog profile add`, `backlog profile list`, `backlog profile use`, `backlog profile set-default`, `backlog profile remove`.

## Default Project

The default project is a project alias used by CLI commands that accept `--project` when that flag is omitted. Set it with `backlog project set-default <alias>`, switch to it with `backlog project use <alias>`, and show it with `backlog project current`.

## Project

A named group of tasks within a workspace. Projects have a unique short `alias` (e.g., `api`, `web`, `security`) that is used in all CLI flags (`-p api`, `--project api`). The alias is also the identifier used in the web UI project selector and in MCP tool calls.

Projects can have a `description` and an optional `repo_path` pointing to a source code directory. They can be archived to hide them from listings without deleting their data.

## Task

The primary unit of work. Every task belongs to a project and carries:

- **Ref**: `TASK-N` where N is a per-workspace sequential integer. Also addressable by bare integer or full ULID.
- **Type**: `task`, `bug`, `issue`, `improvement`, `feature`, `vulnerability`, `chore`, or `spike`
- **Status**: `todo`, `doing`, or `done`
- **Priority**: integer 1–5 where P1 is critical/highest and P5 is backlog/lowest. Default is P3.
- **Actor**: the `kind:name` of the creator (see [Actor](#actor) below)
- **Assignee**: optional free-text name of who is responsible
- **Source**: origin system (e.g., `semgrep`, `pentest-2026`)
- **External ref**: URL or external ticket ID for cross-referencing
- **Project path**: optional file path within the project relevant to the task

Tasks support labels, plans, comments, and attachments.

## Plan

A versioned markdown document attached to a task. Plans capture implementation plans, remediation steps, or any other structured text that may evolve over time.

Every edit to a plan creates a new immutable `plan_version` row. The plan record itself tracks the current version number. Older versions are never modified or deleted — `backlog plan history <plan-id>` returns the full version list.

The first plan on a task is created with `backlog plan add --task TASK-N`. Subsequent edits use `backlog plan update <plan-id>`. An optional `--change-note` documents what changed between versions.

See also [data-model.md](data-model.md) for the `plans` and `plan_versions` table schema.

## Doc

A versioned markdown document attached to a **project** (not a task). Docs are used for longer-form project-level content: architecture overviews, runbooks, design documents, ADRs.

The versioning model is identical to plans: each update creates a new immutable `project_doc_versions` row. Docs also support append operations that create a new version concatenating the old body with new content.

Commands: `backlog doc add`, `backlog doc update`, `backlog doc append`, `backlog doc history`, `backlog doc list`, `backlog doc show`, `backlog doc delete`.

## Memory

A free-form note attached to a project. Unlike docs, memory entries are not versioned — they are mutable records with a body and optional comma-separated tags.

Memory is intended for short-lived context: decisions, assumptions, meeting notes, design rationale. The body supports plain text or markdown. Tags allow filtering: `backlog memory list --project api --tag decision`.

Entries are displayed newest-first. The `backlog memory append` command joins new text onto an existing entry without overwriting it.

## Comment

An actor-attributed note on a task. Comments are append-only (no edit or version history). They are timestamped and attributed to an actor, making them useful for progress notes, review feedback, and completion attestations.

The `/backlog` skill requires agents to post a completion comment before marking a task done.

## Label

A per-project tag that can be attached to tasks. Labels have a name and an optional color. They are scoped to a project — a label named `security` in project `api` is distinct from one in project `web`.

Labels are created with `backlog label create`, attached with `backlog label attach --task TASK-N`, and detached with `backlog label detach`.

## Actor

An actor is a `kind:name` pair that attributes every write operation. Kind is either `human` or `ai`. Examples: `human:alice`, `ai:claude-code`, `ai:semgrep`.

Every task, plan version, doc version, comment, memory entry, and activity log event records the actor. This allows filtering by who (or what) created or modified items: `backlog task list --actor-kind ai`.

The actor is resolved at runtime from the `--as` flag, workspace config, global config, or `$USER` env var in that order. See [architecture.md](architecture.md).

## ULID vs TASK-N

Internally, every entity (task, plan, doc, comment, memory, label, attachment) uses a ULID as its primary key. ULIDs are 26-character lexicographically sortable identifiers generated by `github.com/oklog/ulid/v2`.

Tasks also have a human-readable sequential reference `TASK-N` where N is a monotonically increasing integer per workspace (not per project). This is stored in the `task_seq` column and exposed in JSON as `seq`.

All three formats are accepted wherever a task ID is expected:
- `TASK-5` — canonical human-readable ref used in messages and comments
- `5` — bare integer, resolved to the same task
- `01KR4JA4754H...` — full ULID, useful when chaining JSON output between commands

Plan IDs, doc IDs, memory IDs, and attachment IDs are always ULIDs — there is no sequential N form for those entities.

## Activity log

An append-only event log that records every write operation across the workspace. Each event stores:

- `entity` — the kind of entity affected (task, plan, doc, comment, memory, project, label, attachment)
- `entity_id` — the ULID of the affected entity
- `action` — the operation performed (create, update, move, archive, delete)
- `actor` — `kind:name` of who performed the operation
- `summary` — a human-readable description of the change
- `project_id` — the project the event belongs to (for filtering)
- `payload` — JSON snapshot of the change (for audit purposes)

The activity log is never pruned. It can be queried with `backlog activity` or via the web UI's Activity page. The API endpoint at `GET /api/activity` supports filtering by project and entity kind.
