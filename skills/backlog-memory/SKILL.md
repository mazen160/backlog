---
name: backlog-memory
description: One skill for a backlog project's persistent memory. It does two jobs — learn (load the project's tasks, plans, docs, and memory into the session) and store (synthesize the project's state into persistent memory entries). When invoked without a mode it auto-picks: learn at the start of a fresh session, store after work has been done this session, and it asks when that's ambiguous.
---

# backlog-memory

Two jobs, one skill:

- **learn** — read everything in a project (tasks, plans, docs, memory) into the current session. Read-only.
- **store** — synthesize the project's state into persistent memory entries, grouped by theme, so future sessions load context instantly.

## Invocation

- `/backlog-memory` — auto-pick the mode (see below), default project.
- `/backlog-memory <alias>` — auto-pick the mode for that project.
- `/backlog-memory learn [alias]` — force **learn**.
- `/backlog-memory store [alias]` — force **store**.

## Choosing the mode (when none is given)

An explicit `learn` / `store` always wins. Otherwise decide:

- **Start of a fresh session / orienting** — you've just started, little or no project context is loaded, the user wants to get up to speed → run **learn**.
- **Work has been done this session** — tasks were created, moved, or closed; plans or comments were added; decisions were made; the project state changed → run **store** to refresh the persisted summaries.
- **Ambiguous / you can't tell** — ask the user before doing anything: *"Should I learn (load this project's context) or store (persist a fresh memory snapshot)?"*

When you're unsure whether work has happened, check recent activity and use your judgment:

```sh
backlog activity --project <alias> --limit 20 --json --profile default
```

Recent writes by the current actor in this session lean toward **store**; a quiet log at the start of a session leans toward **learn**.

## Resolve the project

```sh
backlog project list --json --profile default
```

Use the provided alias, or pick the project the user is actively working in. If it's genuinely unclear which project, ask.

---

## Mode: learn (read into context)

The **read phase** — no writes.

### 1. Read everything in parallel

```sh
# Memory entries (decisions, architecture, context notes)
backlog memory list --project <alias> --json --profile default

# Docs list
backlog doc list --project <alias> --json --profile default

# Open tasks (todo + doing)
backlog task list --project <alias> --status todo --json --profile default
backlog task list --project <alias> --status doing --json --profile default
```

For each doc ID, fetch the full body:

```sh
backlog doc show <doc-id> --json --profile default
```

### 2. Surface as context

Present in this order:

- **Memory entries** — grouped by tag, full body of each.
- **Docs** — title + full body of each.
- **Open work** — list tasks as `[TASK-N] P<priority> Title (status)`, doing first, then todo by priority.

### 3. Flag gaps

- No memory entries → suggest running `/backlog-memory store <alias>` to bootstrap them.
- No docs → note it.
- No open tasks → confirm the backlog is clear.

---

## Mode: store (persist synthesized memory)

The **write phase** — read the full workspace and persist synthesized summaries.

### 1. Fetch workspace content

```sh
# All tasks with plans
backlog task list --project <alias> --json --profile default

# Docs (then fetch each body)
backlog doc list --project <alias> --json --profile default
backlog doc show <doc-id> --json --profile default

# Existing memory (to avoid duplicating)
backlog memory list --project <alias> --json --profile default
```

### 2. Synthesize into themes

Distill into these themes — one memory entry per theme, max ~400 words each. Summarise; do not dump raw task lists. Omit themes with no content.

| Theme tag | Content |
|---|---|
| `arch` | Tech stack, data model, key architectural decisions, repo layout |
| `decisions` | Explicit decisions recorded in memory or docs (the "why" behind choices) |
| `open-work` | todo/doing tasks grouped by priority — what still needs doing |
| `done-work` | Recently completed tasks (last 20–30) — what was shipped |
| `context` | Project name/description, actor conventions, workflow norms, anything else |

### 3. Write or update memory entries

For each theme, check whether an entry already exists:

```sh
backlog memory list --project <alias> --tag <theme> --json --profile default
```

**No existing entry** → add:

```sh
backlog memory add "<synthesized body>" \
  --project <alias> \
  --tag "<theme>" \
  --as "ai:<your-model-name>" \
  --profile default
```

**Entry exists** → delete the stale one and re-add (keeps entries fresh without accumulating duplicates):

```sh
backlog memory delete <old-memory-id> --profile default
backlog memory add "<updated body>" \
  --project <alias> \
  --tag "<theme>" \
  --as "ai:<your-model-name>" \
  --profile default
```

### 4. Report

```
backlog-memory (store): <project-alias>
  Tasks read:    <N total> (todo: X, doing: Y, done: Z)
  Docs read:     <N>
  Memory written:
    arch         → <created | updated | skipped (no content)>
    decisions    → <created | updated | skipped>
    open-work    → <created | updated | skipped>
    done-work    → <created | updated | skipped>
    context      → <created | updated | skipped>
```

---

## Notes

- Always use `--profile default` for all backlog CLI calls.
- **learn** is read-only; **store** attributes every write to `ai:<your-model-name>`.
- Do not write raw task JSON into memory — synthesize human-readable summaries.
- store is idempotent: delete-and-recreate per theme keeps entries fresh without duplicates.
- A common flow is learn → do work → store. When in doubt about which the user wants, ask.
