# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v1.0.4 — 2026-07-07

Public release polish for Backlog's website, docs, embedded skills, and web UI.

**Changed**
- Refreshed the public website, README, docs, API pages, and launch assets so the project is ready for a broader public release.
- Consolidated the project-memory skills into a single `backlog-memory` skill. It does both jobs and auto-selects the mode: **learn** (load a project's tasks, plans, docs, and memory into the session) at the start of a fresh session, **store** (synthesize the project's state into persistent memory entries) after work has been done, and it asks when that's ambiguous. Force a mode with `/backlog-memory learn` or `/backlog-memory store`. Replaces the separate `backlog-memory-learn` and `backlog-memory-store` skills.
- Standardized embedded skill files on `SKILL.md` casing for Codex-compatible installs.

**Improved**
- Added first-release launch material for the blog, X/Twitter, LinkedIn, and Hacker News.
- Added new brand, Open Graph, banner, and website imagery used across the public docs and landing pages.
- Added a sidebar collapse toggle in the web UI.

**Fixed**
- Long task titles now wrap correctly in board cards, task headers, and header toolbars.

## v1.0.3 — 2026-05-25

Workflow observability for agent-driven queues. When four agents are closing
tasks in parallel, you need to see whether the work is actually healthy — not
just whether the queue is empty. This release adds two read-only reports that
answer "is this project on track?" and "what did the agents close badly?"
without leaving the terminal.

**Added**
- `backlog activity analyze --project <alias> --since <window>` — a workflow-health
  report for a project over a time window. Surfaces created/completed counts,
  cycle time by task type, status-transition latency (todo→doing, doing→done),
  work-in-progress by actor, reopened work, bug follow-ups, label churn, and the
  human-vs-AI close ratio. `--json` for dashboards; `--since` accepts `7d`,
  `24h`, `all`, RFC3339, or `YYYY-MM-DD`.
- `backlog doctor project --project <alias>` — a project linter that detects
  stale, orphaned, and weakly-closed work: tasks created but never started,
  `doing` tasks gone quiet past `--stale-after` (default `7d`), tasks missing
  plans, tasks closed with no completion comment or evidence, label-only latest
  activity, and final-audit tasks marked done while earlier work is still open.
  Each issue ships with a severity, a code, and the evidence behind it.

**Improved**
- `backlog install-skills` now installs Codex skills into `~/.codex/skills/<name>/SKILL.md`
  with Codex-compatible frontmatter, instead of writing saved prompts.
- The Docs web UI can download all visible docs from the list view in one click.

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
