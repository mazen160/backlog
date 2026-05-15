---
description: Read a backlog workspace and synthesize its content into persistent Claude memory entries — tasks, plans, docs, and existing memory — grouped by theme so future sessions load context instantly.
---

# backlog-memory

Ingest a backlog project workspace into structured Claude memory entries. Run this once per project to bootstrap context, or re-run to refresh stale entries.

## When to use

Invoke as `/backlog-memory` or `/backlog-memory <project-alias>`.

- No argument: uses the default profile's default project (or prompts if ambiguous).
- With alias: targets that specific project (e.g. `/backlog-memory api`).

## Workflow

### 1. Resolve the project

```sh
backlog project list --json --profile default
```

If an alias was provided, use it. If not, pick the project the user is actively working in (check `state.project` or ask).

### 2. Fetch all workspace content

Run all three in parallel:

```sh
# All tasks (with plans and comments)
backlog task list --project <alias> --json --profile default

# All docs (latest version of each)
backlog doc list --project <alias> --json --profile default

# Existing memory entries
backlog memory list --project <alias> --json --profile default
```

For each doc ID returned, fetch the full body:
```sh
backlog doc show <doc-id> --json --profile default
```

For each task with plans, the plan bodies are included in `task list` output — no extra fetch needed.

### 3. Synthesize into themes

Group the collected content into these themes (omit any that have no content):

| Theme tag | What goes in it |
|---|---|
| `arch` | Stack, data model, key design decisions extracted from docs and memory |
| `decisions` | Explicit decision records found in memory entries or doc sections |
| `open-work` | Summary of todo/doing tasks, grouped by priority |
| `done-work` | Summary of recently completed tasks (last 20) |
| `context` | Project description, repo path, actor conventions, anything that doesn't fit above |

Write one memory entry per theme. Keep each body under ~400 words — summarise, don't dump raw task lists.

### 4. Write memory entries

Before writing, check whether entries with the same tags already exist:

```sh
backlog memory list --project <alias> --tag <theme> --json --profile default
```

- If an entry exists for that theme → **append** updated content (use `backlog memory append <id> "..."`)
- If none exists → **add** a new entry

```sh
# Add (new)
backlog memory add "<synthesized body>" \
  --project <alias> \
  --tag "<theme>" \
  --as "ai:<your-model-name>" \
  --profile default

# Append (existing)
backlog memory append <memory-id> "<updated content>" --profile default
```

### 5. Report

After writing, output a short summary:

```
backlog-memory: <project-alias>
  Tasks read:   <N> (todo: X, doing: Y, done: Z)
  Docs read:    <N>
  Themes written:
    arch        → <new | updated>
    decisions   → <new | updated>
    open-work   → <new | updated>
    done-work   → <new | updated>
    context     → <new | updated>
```

## Notes

- Always use `--profile default` for all backlog CLI calls.
- Always attribute writes to `ai:<your-model-name>`.
- Do not write raw task JSON into memory — synthesize human-readable summaries.
- Idempotent: re-running should update existing entries, not create duplicates.
- If the project has no tasks, docs, or memory, write a single `context` entry with the project description and exit cleanly.
