# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## v1.0.3 — 2026-05-25

**Added**
- `backlog activity analyze --project <alias> --since <window>` summarizes project activity with created/completed counts, cycle time by type, status-transition latency, WIP by actor, weak completion evidence, reopened work, bug followups, label churn, and human-vs-AI close ratios.
- `backlog doctor project --project <alias>` detects stale, orphaned, and weakly closed project work, including never-started tasks, stale `doing` tasks, missing plans, missing completion comments/evidence, label-only latest activity, and final-audit tasks closed while earlier work remains open.

**Improved**
- `backlog install-skills` now installs Codex skills into `~/.codex/skills/<name>/SKILL.md` with Codex-compatible frontmatter instead of writing saved prompts.
- The Docs web UI can download all visible docs from the list view.

**Fixed**
- Hide the all-docs download action while a single document is open in the reader.

## v1.0.2 — 2026-05-16

**Improved**
- Render task comment bodies as markdown in the web UI task detail view.
- Fix the web UI markdown renderer so fenced code blocks and inline code preserve literal markdown syntax instead of rendering headings, bold text, or other formatting inside code.

## v1.0.0 — Initial release

A local-first task queue your AI coding agents can read and write directly.
Single binary, no server, no SaaS.

**Workspace**
- `backlog init` creates `backlog.db` (SQLite, WAL mode) and `config.toml`.
- Profiles registered in `~/.config/backlog/config.toml`; switch with `--profile`.
- DB resolution chain: `--db` → `$BACKLOG_DB` → `--profile` → default profile.
- `backlog doctor check` runs `PRAGMA integrity_check`; `doctor backup` does an atomic `VACUUM INTO`.

**Tasks, plans, comments**
- `backlog task add/list/show/update/move/archive/delete` with human-readable `TASK-N` refs, full ULIDs, or bare integers.
- Types `task`/`bug`/`issue`/`improvement`/`feature`/`vulnerability`/`chore`/`spike`/`bucket-list`; statuses `todo`/`doing`/`done`; priorities P1–P5.
- FTS5 full-text search across title and description with prefix (`sql*`) and boolean (`jwt OR csrf`) support.
- Plans are versioned markdown: every edit creates a new immutable version, full history queryable via `plan history`.
- Comments are append-only and actor-attributed.

**Project knowledge**
- `backlog memory add/list/append` — tagged free-form notes for cross-session agent scratchpads.
- `backlog doc add/list/show/update/history` — versioned project documentation.
- `backlog attachment add/list/fetch/delete` — binary files stored inside the SQLite DB.
- `backlog label create/list/attach/detach` — per-project tags with optional hex color.

**Actor attribution**
- Every write takes `--as kind:name` (e.g. `ai:claude-code`, `human:alice`).
- Actor stored as a `(kind, name)` pair at the DB level with a `CHECK` constraint, exposed on every row in the activity log.
- Defaults to `human:$USER` when `--as` is omitted.

**Import / export**
- `backlog import-findings <file.json>` bulk-creates tasks from structured findings (security scanners, AI triage agents); supports inline plans per item; `--dry-run` available.
- `backlog import <other.db>` copies tasks, plans, comments, and labels from another workspace.
- `backlog export --format json|csv|md` with optional `--project` and `--out` flags.

**Surfaces**
- **CLI** — every command supports `--json` for machine-readable output.
- **HTTP API** — `backlog web` starts an embedded server with a Notion-style SPA (tasks, board, grid, plans, docs, memory, attachments, activity).
- **MCP server** — `backlog mcp serve` exposes the operations as JSON-RPC 2.0 tools over stdio. Compatible with Claude Code, Cursor, Codex, and OpenCode.
- **Skills** — four agentic-loop skills (`backlog`, `backlog-enhance-tasks`, `backlog-loop`, `backlog-goal`) embedded in the binary. `backlog install-skills` writes them into `~/.claude`, `~/.cursor`, `~/.config/opencode`, and `~/.codex` in each tool's native format.

**Developer experience**
- `backlog completion bash|zsh|fish|powershell` — shell completion scripts.
- `backlog activity` — append-only audit trail of every state transition.
- `backlog schema` — JSON Schema for `task`, `findings_file`, `manifest`, and `actor` payloads.
- `backlog version` — version injected via ldflags at build time.
