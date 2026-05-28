---
name: backlog-enhance-tasks
description: Enhance a backlog task's title, description, and optionally generate a plan using Claude. Invoke as /backlog-enhance-tasks TASK-N or /backlog-enhance-tasks TASK-N --build-plan.
---

# backlog-enhance-tasks

Enhance a backlog task's title, description, and optionally generate a plan using Claude.

## When to use

Invoke as `/backlog-enhance-tasks TASK-N` or `/backlog-enhance-tasks TASK-N --build-plan`.

Accepts: a task ref (`TASK-N` or bare integer), optional `--build-plan` flag.

## Workflow

### 1. Fetch the task

```sh
./backlog --profile default task show <ref> --json
```

Parse the JSON. Extract: `id`, `seq`, `title`, `description`, `type`, `priority`, `project`.

### 2. Rewrite the title

Improve the title for clarity and specificity. Rules:
- Use imperative verb ("Add", "Fix", "Remove", "Expose", "Migrate" — not "Adding" or "Added")
- Be specific: include the subsystem or file if known (e.g., "Fix circular onclick rebind in load-more button" not "Fix button bug")
- Keep it under 80 chars
- Do not change the intent or scope

### 3. Expand the description

Rewrite the description as structured markdown with these sections (omit sections that don't apply):

```markdown
## Context
<1-2 sentences on why this matters — what breaks or is missing without it>

## Acceptance criteria
- [ ] <specific, testable criterion>
- [ ] <specific, testable criterion>
- [ ] ...

## Implementation hints
<optional: file paths, function names, API patterns — only if clearly known>
```

Rules:
- Keep what's already correct in the existing description
- Do not invent scope that isn't implied by the title/type/context
- Write for a developer who hasn't seen this codebase before

### 4. Write back to the task

```sh
./backlog --profile default task update <ref> \
  --title "<improved title>" \
  --description "<expanded description>" \
  --as "ai:claude-sonnet-4-6"
```

### 5. Build plan (if `--build-plan` flag provided)

Generate a concise implementation plan:

```markdown
## Steps
1. <first concrete action — file, function, what to change>
2. <next action>
...

## Testing
- <how to verify this works>
```

Then attach it:
```sh
./backlog --profile default plan add \
  --task <ref> \
  --title "Implementation plan" \
  --content "<plan markdown>" \
  --as "ai:claude-sonnet-4-6"
```

## Output

After writing back, show a summary:

```
Enhanced TASK-N: <new title>
  Title:       <old> → <new>
  Description: <N lines added>
  Plan:        <attached v1 | skipped>
```

## Notes

- Always use `--profile default` for all backlog CLI calls in this project
- Always attribute writes to `ai:claude-sonnet-4-6` (or the current model)
- Do not change `type`, `priority`, `status`, or `project` — only `title` and `description`
- If the task already has a detailed description, preserve its structure and only enrich it
