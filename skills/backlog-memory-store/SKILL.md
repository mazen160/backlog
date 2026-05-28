---
name: backlog-memory-store
description: Synthesize a backlog project's tasks, docs, and activity into persistent memory entries so future sessions load context instantly. The write counterpart to /backlog-memory-learn.
---

# backlog-memory-store

Read the full backlog workspace and write synthesized memory entries grouped by theme. Run `/backlog-memory-learn` first if you need the content in context — this skill focuses on persisting it.

## When to use

Invoke as `/backlog-memory-store` or `/backlog-memory-store <project-alias>`.

- No argument: uses the default profile's active project.
- With alias: targets that project (e.g. `/backlog-memory-store api`).
- Re-run any time the project state changes significantly to refresh stale entries.

## Workflow

### 1. Resolve the project alias

```sh
backlog project list --json --profile default
```

### 2. Fetch workspace content

```sh
# All tasks with plans
backlog task list --project <alias> --json --profile default

# Docs
backlog doc list --project <alias> --json --profile default
# Then for each doc ID:
backlog doc show <doc-id> --json --profile default

# Existing memory (to avoid duplicating)
backlog memory list --project <alias> --json --profile default
```

### 3. Synthesize into themes

Distill the raw content into these themes. Write one memory entry per theme, max ~400 words each. Summarise — do not dump raw task lists.

| Theme tag | Content |
|---|---|
| `arch` | Tech stack, data model, key architectural decisions, repo layout |
| `decisions` | Explicit decisions recorded in memory or docs (the "why" behind choices) |
| `open-work` | todo/doing tasks grouped by priority — what still needs doing |
| `done-work` | Recently completed tasks (last 20–30) — what was shipped |
| `context` | Project name/description, actor conventions, workflow norms, anything else |

Omit themes that have no content.

### 4. Write or update memory entries

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

**Entry exists** → delete the stale one and re-add (cleaner than appending):
```sh
backlog memory delete <old-memory-id> --profile default
backlog memory add "<updated body>" \
  --project <alias> \
  --tag "<theme>" \
  --as "ai:<your-model-name>" \
  --profile default
```

### 5. Report

```
backlog-memory-store: <project-alias>
  Tasks read:    <N total> (todo: X, doing: Y, done: Z)
  Docs read:     <N>
  Memory written:
    arch         → <created | updated | skipped (no content)>
    decisions    → <created | updated | skipped>
    open-work    → <created | updated | skipped>
    done-work    → <created | updated | skipped>
    context      → <created | updated | skipped>
```

## Notes

- Always use `--profile default` for all backlog CLI calls.
- Always attribute writes to `ai:<your-model-name>`.
- Idempotent: delete-and-recreate keeps entries fresh without accumulating duplicates.
- Pair with `/backlog-memory-learn` to both load context and persist it in one flow.
