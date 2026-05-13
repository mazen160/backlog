# Data Model

## SQLite schema overview

All data lives in a single `backlog.db` SQLite file. The schema is managed by sequential migration files in `internal/migrate/migrations/` and applied automatically when the database is opened.

| Table | Purpose |
|---|---|
| `projects` | Named project containers |
| `tasks` | Individual units of work |
| `plans` | Plan headers (one per plan, links to `plan_versions`) |
| `plan_versions` | Immutable plan version content |
| `comments` | Actor-attributed notes on tasks |
| `labels` | Per-project tags |
| `task_labels` | Many-to-many join between tasks and labels |
| `project_memory` | Free-form project-scoped notes |
| `project_docs` | Doc headers (links to `project_doc_versions`) |
| `project_doc_versions` | Immutable doc version content |
| `attachments` | Binary files attached to tasks or docs (data stored as BLOB) |
| `activity_log` | Append-only write event log |
| `tasks_fts` | FTS5 virtual table for full-text search on task title + description |
| `schema_meta` | Tracks current schema version for migration idempotency |

## `tasks` table

Key columns:

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT | ULID, primary key |
| `task_seq` | INTEGER | Per-workspace sequential ref (TASK-N). Added in migration 0002. |
| `project_id` | TEXT | FK to `projects.id`, CASCADE DELETE |
| `title` | TEXT | Short label |
| `description` | TEXT | Markdown body (default `''`) |
| `type` | TEXT | Enum: `task`, `bug`, `issue`, `improvement`, `feature`, `vulnerability`, `chore`, `spike` |
| `status` | TEXT | Enum: `todo`, `doing`, `done` |
| `priority` | INTEGER | 1 (highest) – 5 (lowest), default 3 |
| `assignee` | TEXT | Free-text name (default `''`) |
| `due_at` | INTEGER | Unix nanoseconds (nullable) |
| `actor_kind` | TEXT | `human` or `ai` |
| `actor_name` | TEXT | Name portion of actor |
| `source` | TEXT | Origin system (e.g., `semgrep`) |
| `external_ref` | TEXT | URL or external ticket ID |
| `project_path` | TEXT | File path within project (added migration 0007) |
| `completed_at` | INTEGER | Unix nanoseconds (nullable, set when status → done) |
| `archived_at` | INTEGER | Unix nanoseconds (nullable, soft delete) |
| `created_at` | INTEGER | Unix nanoseconds |
| `updated_at` | INTEGER | Unix nanoseconds |

Indexes: `(project_id, status)`, `priority`, `type`, `(actor_kind, actor_name)`, unique `task_seq`.

FTS5 triggers (`tasks_ai`, `tasks_ad`, `tasks_au`) keep `tasks_fts` in sync with inserts, deletes, and updates automatically.

## `plans` and `plan_versions` tables

Plans use a two-table versioning model. The `plans` table is the header record; the `plan_versions` table holds the actual content and is append-only.

**`plans`:**

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT | ULID, primary key |
| `task_id` | TEXT | FK to `tasks.id`, CASCADE DELETE |
| `current_version` | INTEGER | Points to the latest `plan_versions.version` value |
| `source` | TEXT | Origin (default `''`) |
| `archived_at` | INTEGER | Nullable, Unix nanoseconds |
| `created_at` | INTEGER | Unix nanoseconds |
| `updated_at` | INTEGER | Unix nanoseconds |

**`plan_versions`:**

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT | ULID, primary key |
| `plan_id` | TEXT | FK to `plans.id`, CASCADE DELETE |
| `version` | INTEGER | Monotonically increasing per plan; UNIQUE(plan_id, version) |
| `title` | TEXT | Version title |
| `body` | TEXT | Markdown content |
| `actor_kind` | TEXT | `human` or `ai` |
| `actor_name` | TEXT | |
| `change_note` | TEXT | Optional human-readable diff summary |
| `created_at` | INTEGER | Unix nanoseconds |

Every `plan update` increments `current_version` and inserts a new `plan_versions` row. Old versions are never modified.

## `project_docs` and `project_doc_versions` tables

Identical versioning model to plans, but scoped to projects rather than tasks.

**`project_docs`:**

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT | ULID, primary key |
| `project_id` | TEXT | FK to `projects.id`, CASCADE DELETE |
| `title` | TEXT | |
| `current_version` | INTEGER | Latest version number |
| `actor_kind` / `actor_name` | TEXT | Creator actor |
| `archived_at` | INTEGER | Nullable |
| `created_at` / `updated_at` | INTEGER | Unix nanoseconds |

**`project_doc_versions`:**

Same structure as `plan_versions` with `doc_id` instead of `plan_id`, and a `UNIQUE(doc_id, version)` constraint.

## `activity_log` table

Append-only event log. Rows are never updated or deleted.

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT | ULID, primary key |
| `project_id` | TEXT | Project scope for filtering (added migration 0008, default `''`) |
| `entity` | TEXT | Entity kind: `task`, `plan`, `doc`, `comment`, `memory`, `project`, `label`, `attachment` |
| `entity_id` | TEXT | ULID of the affected entity |
| `action` | TEXT | Operation: `create`, `update`, `move`, `archive`, `delete` |
| `summary` | TEXT | Human-readable description (added migration 0008, default `''`) |
| `actor_kind` / `actor_name` | TEXT | Actor who performed the operation |
| `payload` | TEXT | JSON snapshot of the change (default `{}`) |
| `created_at` | INTEGER | Unix nanoseconds |

Indexes: `(entity, entity_id)`, `(project_id, created_at DESC)`.

## Migration approach

Migration files live in `internal/migrate/migrations/` and are named `NNNN_description.sql`. They are embedded into the binary at compile time via `//go:embed migrations/*.sql` in `internal/migrate/migrate.go`.

On every DB open, `migrate.Run(db)` is called. It:

1. Creates `schema_meta` if it does not exist.
2. Reads the current `schema_version` from `schema_meta` (0 if absent).
3. Lists all `*.sql` files sorted lexicographically.
4. Runs each file whose 1-based index exceeds the current version, in a transaction.
5. Updates `schema_version` after each successful migration.

Migrations are applied exactly once, in order. They are never reversed. Adding a new migration: create `internal/migrate/migrations/NNNN_description.sql` with the next sequential number.

Current migrations:

| File | Change |
|---|---|
| `0001_init.sql` | Initial schema: projects, tasks, plans, plan_versions, comments, labels, task_labels, activity_log, tasks_fts, schema_meta |
| `0002_task_seq.sql` | Adds `task_seq` column and unique index; backfills sequential values |
| `0003_memory.sql` | Adds `project_memory` table (original key/value design) |
| `0004_docs.sql` | Adds `project_docs` and `project_doc_versions` tables |
| `0005_attachments.sql` | Adds `attachments` table with BLOB storage |
| `0006_memory_v2.sql` | Redesigns `project_memory`: replaces key/value with free-form `body` + `tags` |
| `0007_task_project_path.sql` | Adds `project_path` column to tasks |
| `0008_activity_project_summary.sql` | Adds `project_id` and `summary` columns to `activity_log` |

## Timestamp format

All timestamps are stored as `INTEGER` values representing Unix time in **nanoseconds** (not seconds or milliseconds). The Go standard library `time.Now().UnixNano()` is used for all writes.

## ULID generation

Primary keys for all entities are ULIDs generated by `github.com/oklog/ulid/v2`. ULIDs are 128-bit values encoded as 26-character Base32 strings. They are:

- Lexicographically sortable by creation time (millisecond precision)
- URL-safe and case-insensitive
- Compatible with UUIDs in storage size

The random component uses a monotonic entropy source to guarantee uniqueness even when multiple ULIDs are generated in the same millisecond.

The `task_seq` integer (TASK-N) is a separately maintained sequential counter per workspace, derived from insertion order. It is more human-friendly than a ULID but is not a primary key.
