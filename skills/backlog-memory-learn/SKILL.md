---
name: backlog-memory-learn
description: Load a backlog project's full context into the session — tasks, plans, docs, and memory entries — so the agent can work with complete situational awareness without re-deriving what is already known.
---

# backlog-memory-learn

Read everything stored in a backlog project and surface it as session context. This is the **read phase** — no writes. Run it at the start of a session or whenever you need a full picture of the project state.

## When to use

Invoke as `/backlog-memory-learn` or `/backlog-memory-learn <project-alias>`.

- No argument: uses the default profile's active project.
- With alias: targets that project (e.g. `/backlog-memory-learn api`).

## Workflow

### 1. Resolve the project alias

```sh
backlog project list --json --profile default
```

Use the provided alias, or pick the project the user is actively working in.

### 2. Read everything in parallel

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

### 3. Surface as context

Present the content in this order:

**Memory entries** — grouped by tag. Show full body of each entry.

**Docs** — show title + full body of each doc.

**Open work** — list tasks as:
```
[TASK-N] P<priority> Title (status)
```
Group by status: doing first, then todo by priority.

### 4. Flag gaps

If memory entries are empty → suggest running `/backlog-memory-store <alias>` to bootstrap them.

If no docs exist → note it.

If no open tasks exist → confirm the backlog is clear.

## Notes

- Read-only. No writes, no side effects.
- Always use `--profile default`.
- Do not truncate memory entries or doc bodies — show them in full.
- This skill feeds `/backlog-memory-store`: learn first, then store if you want to refresh the persisted summaries.
