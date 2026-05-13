---
name: backlog-loop
description: Pick up one task from a backlog project and iterate on it until a Judge sub-agent verifies it is genuinely done, then exit. The "loop" is internal to a single task — implement, verify, judge, fix, re-judge — not across multiple tasks. Invoke as `/backlog-loop <project>` to pull the next highest-priority `todo` task, or `/backlog-loop <project> <ref>` to target a specific task. Use when the user wants the agent to clear one item off the backlog with a built-in verification gate, without engaging the full goal-and-checkpoint workflow of /backlog-goal.
---

# backlog-loop

One task in. Iterate until a skeptical Judge says it's actually done. Exit.

The name is a callback to the Ralph loop: the loop is on a single task — implement → verify → judge → fix → re-judge — bounded by a max iteration count. This skill never picks up a second task.

## Requires: the `backlog` skill

Load `skills/backlog/skill.md` first. It is the canonical reference for every CLI flag, command, ID format (`TASK-N` / bare integer / ULID), JSON shape, and enum value. This skill assumes you know how to drive the CLI.

In this repo, always pass `--profile default` (per `docs/CLAUDE/MEMORY.md` — `go test` can pollute the profile registry) and `--as ai:claude-opus-4-7` (or the current model's exact ID) on every write.

## Invocation

```
/backlog-loop <project>          # pull next todo task from <project>, iterate to done, exit
/backlog-loop <project> <ref>    # work a specific task (TASK-N, bare integer, or ULID)
/backlog-loop help               # print the headless-execution help and exit
```

If the user omits `<project>` (and didn't pass `help`), list available projects and ask which one — do not guess:

```sh
./backlog --profile default project list --json
```

If the user passes `help`, jump to the **Headless execution** section below, print it, and exit. Do not pick up a task.

## Hard rules

- **One task only.** The skill never moves to a second task. The user invokes it again if they want more.
- **Mandatory Judge gate.** The task only moves to `done` after a `[JUDGE RECEIPT]` says PASS. The implementing agent never self-marks `done`.
- **Max 5 attempts.** Implement → verify → judge counts as one attempt. After 5 failed Judge verdicts, stop and mark the task blocked with a full diagnosis.
- **Never expand scope silently.** Work discovered mid-execution that's outside the task → new task, not silent expansion of this one.
- **Always attribute writes.** `--as ai:claude-opus-4-7` on every write.
- **Always `--profile default`** in this repo.

## Workflow

### 1. Select the task

If a ref was provided:

```sh
./backlog --profile default task show <ref> --json
```

Otherwise:

```sh
./backlog --profile default task list --project <project> --status todo --sort priority --json
```

Pick the **first** task. Sort `priority` orders P1 → P5; within priority, lower `seq` first. If the result is empty, tell the user "no todo tasks in `<project>`", show what's `doing`, and exit. Do not invent work.

Capture from the JSON: `id` (ULID), `seq` (`TASK-N`), `title`, `description`, `type`, `priority`, `labels`, any attached `plans`.

### 2. Check the description is judgeable

The Judge needs verifiable acceptance criteria to do its job. Read the description fully and decide whether it contains them.

**Sufficient:** explicit acceptance criteria (a checklist, or sentences naming the observable outcome) AND at least one verification approach (a command, a check, or "manual: <what to inspect>"). The structured shape produced by `backlog-enhance-tasks` (Context / Acceptance criteria / Implementation hints / Verification) is the ideal.

**Insufficient:** empty description, one-line title-only, or vague language with no observable signal of done.

If insufficient, **stop**. Post a comment and exit without picking up:

```sh
./backlog --profile default comment add \
  "Cannot pick up via /backlog-loop: description has no judgeable acceptance criteria. Missing: <criteria | verification commands | both>. Suggest running /backlog-enhance-tasks TASK-N first." \
  --task TASK-N --as ai:claude-opus-4-7
```

This is non-negotiable. A loop without a Judge gate is the failure mode this skill exists to prevent; a Judge without criteria is theater.

### 3. Move to doing

```sh
./backlog --profile default task move TASK-N --status doing --as ai:claude-opus-4-7
./backlog --profile default comment add \
  "Picked up via /backlog-loop. Starting attempt 1 of max 5." \
  --task TASK-N --as ai:claude-opus-4-7
```

### 4. Attach a plan if non-trivial

A task is non-trivial if any of: touches more than one file, mixes concerns, has more than two acceptance criteria, or requires reasoning that a future reader would want to see.

```sh
./backlog --profile default plan add \
  --task TASK-N --title "Implementation plan" \
  --content "$(cat <<'EOF'
## Steps
1. <first concrete action — file, function, what to change>
2. <next action>

## Testing
- <how to verify against each acceptance criterion>

## Risks
- <risk + mitigation>
EOF
)" --as ai:claude-opus-4-7 --json
```

Skip the plan for trivial single-file edits.

### 5. The iterate-until-judged loop

Run up to **5 attempts**. Each attempt is one full pass through implement → verify → judge.

```
for attempt in 1..5:
  implement (or refine based on the prior Judge receipt)
  run verification commands
  dispatch Judge sub-agent
  if Judge says PASS: break, go to step 6
  if Judge says FAIL: read the failure, plan the fix, continue
after 5 failures: go to step 7 (blocked)
```

**5a. Implement (attempt N)**

If `N == 1`: implement from scratch against the description and any attached plan.

If `N > 1`: read the prior `[JUDGE RECEIPT]` from the task's comments. The Judge's `criteria` list (each marked PASS or FAIL) and `verification_commands` output tell you exactly what to fix. Don't rebuild what already passes. Don't argue with the Judge — fix the failing criterion.

For larger work, delegate to a sub-agent via the `Agent` tool (`general-purpose` for implementation, `Explore` for research). Pass the task description, the failing criteria from the last Judge receipt (if any), and the verification command. The sub-agent must not move the board.

Drop progress comments at meaningful milestones:

```sh
./backlog --profile default comment add \
  "Attempt N: edited internal/web/server.go:142, added test in internal/web/server_test.go:88." \
  --task TASK-N --as ai:claude-opus-4-7
```

**5b. Run verification**

Run every verification command listed in the description. Capture stdout/stderr and exit codes. If a command fails, attempt a fix (up to 2 tries) before handing the result to the Judge. If verification is "manual: <inspect X>", note that and let the Judge handle it.

**5c. Dispatch the Judge**

Use the `Agent` tool with `subagent_type: "general-purpose"` and a fully self-contained prompt. The Judge has no memory of this session.

Judge prompt template (paste verbatim with substitutions):

```
You are the Judge for backlog task TASK-N in project <project>. This is attempt N of 5.

Read the task description in full:
  ./backlog --profile default task show TASK-N --json | jq -r '.description'

Your job:

1. For EACH acceptance criterion in the description, determine PASS or FAIL by inspecting the actual repo state. Read files. Cite file:line evidence for each verdict.
2. Run EVERY verification command listed in the description. Capture stdout/stderr and exit code. Non-zero exit is a FAIL unless the description explicitly says otherwise.
3. Look for proxy signals being mistaken for completion — flag any of these and FAIL the corresponding criterion:
   - "Tests pass" without the test actually exercising the criterion
   - "Files changed" without behavior changed
   - "Plan written" instead of implemented
   - "Build succeeded" without the output behaving correctly
   - "Lint clean" — orthogonal to the requirement
   - The implementing agent's own claim of done
4. Look for adjacent breakage — things outside the criteria that a reasonable reviewer would flag (a broken test elsewhere, an obvious bug introduced). Note these as `judge_observations`. They do not fail the task on their own but must be reported.
5. Return a verdict: PASS or FAIL.

Hard rules:
- You are READ-ONLY. Do not edit files. Run only verification commands and read commands.
- Be skeptical by default. A criterion is FAIL until evidence convincingly shows PASS.
- If a criterion is unverifiable as written, FAIL it and explain what the criterion would need to say to be verifiable.
- Do not move the task on the board. Return only the receipt.

Output format (exact, parseable):

[JUDGE RECEIPT]
result: done
attempt: <N>
verdict: PASS | FAIL
criteria:
  - [PASS|FAIL] <criterion text> — <evidence: file:line, command exit code, or quoted output>
  - ...
verification_commands:
  - `<command>` → exit <code>
    <relevant output snippet>
  - ...
judge_observations:
  - <optional: adjacent issues outside the criteria>
reason: <one paragraph rationale for the verdict>
next_fix_hint: <if FAIL: one sentence telling the implementing agent what to focus on next>
```

**5d. Post the Judge receipt**

Post the full Judge output as a comment, prefixed exactly so the next iteration can parse it:

```sh
./backlog --profile default comment add "$(cat <<'EOF'
<paste the Judge's full output verbatim, including the [JUDGE RECEIPT] header>
EOF
)" --task TASK-N --as ai:claude-opus-4-7
```

**5e. Branch on the verdict**

- **PASS:** break out of the loop. Go to step 6.
- **FAIL:** record the iteration and continue the loop with attempt N+1. Read `next_fix_hint` and the failing criteria — those drive the next attempt's implementation.

### 6. Complete

When the Judge returns PASS:

```sh
./backlog --profile default comment add "$(cat <<'EOF'
Completed via /backlog-loop. Judge approved on attempt <N>.

Changed:
- <file:line summary>
- <file:line summary>

Verified:
- `<verify command>` → exit 0

Follow-ups spawned:
- TASK-M (<title>) — if any
EOF
)" --task TASK-N --as ai:claude-opus-4-7

./backlog --profile default task move TASK-N --status done --as ai:claude-opus-4-7
```

Print a one-line summary to the user and **stop**:

```
Worked TASK-N (<title>) → done. Judge passed on attempt <N>. Verified: <command> exit 0.
```

### 7. Blocked (max attempts reached or hard blocker)

If the Judge has rejected 5 consecutive attempts, or you hit a real blocker (needs credentials, a product decision, a destructive op, a criterion that is unverifiable as written and the user must clarify), do **not** mark `done`. Move back to `todo` with a full diagnosis:

```sh
./backlog --profile default comment add "$(cat <<'EOF'
Blocked after <N> attempt(s) via /backlog-loop.

Why blocked:
<one paragraph — what kept failing, or what's needed from outside>

Failing criteria after the last Judge pass:
- <criterion> — <evidence the Judge cited>

What I tried:
- Attempt 1: <one-line summary>
- Attempt 2: <one-line summary>
- ...

What's needed to unblock:
- <specific action by the user or another task>

Suggested next step:
- Sharpen the acceptance criteria (run /backlog-enhance-tasks TASK-N)
- OR split into a smaller task with a tighter scope
- OR provide <credential | decision | reference> and re-run /backlog-loop
EOF
)" --task TASK-N --as ai:claude-opus-4-7

./backlog --profile default task move TASK-N --status todo --as ai:claude-opus-4-7
```

Print:

```
TASK-N (<title>) → blocked after <N> attempts. See comment for diagnosis.
```

Stop.

### 8. Discovery during execution

If during any attempt you find related work outside the task's scope — a missing test elsewhere, a refactor that would help, a broken adjacent function — **create a new task** rather than expanding this one:

```sh
./backlog --profile default task add --project <project> \
  --title "<imperative + specific>" \
  --description "Spawned from TASK-N. <context>" \
  --type <task|bug|chore> --priority <P1-P5> \
  --as ai:claude-opus-4-7 --json
```

Comment on TASK-N linking the new TASK-M. The Judge does not consider spawned tasks part of TASK-N's completion criteria.

## Why a Judge for a single task

Without the Judge, this skill is just "do one task and exit" — the agent self-marks `done` and the same laziness failure mode from the Codex Goal lessons applies (tests pass ≠ feature works; files changed ≠ behavior changed). The Judge enforces that completion is observable, not asserted.

The Judge is much lighter than the one in `/backlog-goal` — one task, one set of criteria, no cross-checkpoint state. But the contract is the same: read-only, evidence-based, skeptical default, reject proxy signals, never picks the next active task.

## Headless execution

This is what `/backlog-loop help` prints. `backlog-loop` is designed to drain cleanly without prompts — pair it with `claude -p` (print mode, non-interactive) to run as a cron job, CI step, or background daemon.

### Prerequisites

- `ANTHROPIC_API_KEY` set in the environment (or `claude /login` already run on the box).
- `claude` CLI installed and on PATH (`claude --version` works).
- The `backlog` binary built and on PATH, OR run from a checkout where `./backlog` exists. The backlog skill at `skills/backlog/skill.md` must be reachable by Claude Code (project-level skills are auto-discovered from `skills/` in this repo).
- A workspace registered as a backlog profile (`backlog profile list`). Default is `default`. Override with `BACKLOG_DB` env var to bypass profile resolution entirely.

### Single-task headless run

```sh
claude -p "/backlog-loop <project>" \
  --permission-mode acceptEdits \
  --output-format stream-json \
  --max-turns 80
```

Flag notes:

- **`-p`** — print mode, non-interactive. Required for headless.
- **`--permission-mode acceptEdits`** — auto-approve file edits the skill needs to make. Use `bypassPermissions` only if you also trust the skill to run arbitrary shell (it will need to run `make test`, `pytest`, etc. for verification). For production cron, prefer a tighter allowlist with `--allowed-tools`.
- **`--output-format stream-json`** — every event as a JSON line. Parseable. Switch to `text` for human-readable logs, `json` for a single final blob.
- **`--max-turns 80`** — guardrail. A typical task is well under 30 turns; 80 leaves headroom for 5 Judge iterations on a complex task without runaway.
- Add **`--model claude-opus-4-7`** (or the model you want) if the default isn't right for the workload.
- Add **`--cwd /path/to/repo`** if you're invoking from outside the repo.

The skill exits cleanly in three terminal states, all with exit code `0`:

| Outcome | What you'll see in output | Recovery |
|---|---|---|
| Task → done | "Worked TASK-N → done. Judge passed on attempt N." | Pick up another with a fresh invocation. |
| Task → blocked | "TASK-N → blocked after N attempts. See comment for diagnosis." | Read the task's comments, fix the underlying issue, re-run. |
| No work | "No todo tasks in `<project>`." | Add more tasks or stop the cron. |

Non-zero exit means a harness error (API failure, permissions denied, claude CLI crash, missing skill) — not a task failure. Always log to a file:

```sh
claude -p "/backlog-loop <project>" \
  --permission-mode acceptEdits \
  --output-format text \
  --max-turns 80 \
  > /var/log/backlog-loop-$(date +%Y%m%d-%H%M%S).log 2>&1
```

### Draining a project (work every task until empty)

The skill itself never picks up a second task — the outer loop is your responsibility. Pattern:

```sh
#!/usr/bin/env bash
set -euo pipefail

PROJECT="${1:?usage: $0 <project>}"
MAX_INVOCATIONS=20  # safety cap, change as needed

for i in $(seq 1 "$MAX_INVOCATIONS"); do
  remaining=$(./backlog --profile default task list --project "$PROJECT" --status todo --json | jq '.tasks | length')
  echo "[drain $i] $remaining todo task(s) in $PROJECT"
  if [ "$remaining" -eq 0 ]; then
    echo "drained."
    break
  fi
  claude -p "/backlog-loop $PROJECT" \
    --permission-mode acceptEdits \
    --output-format text \
    --max-turns 80
done
```

Notes:
- The `--profile default` flag (and the binary at `./backlog`) assume you're inside this repo. Adjust for other layouts.
- Each invocation is a fresh Claude session — there is no cross-invocation memory beyond what the skill wrote to the backlog DB. That's the design: the DB is the persistent state.
- Tasks that go to `blocked` will be skipped on the next iteration (they're back in `todo` but the comments document why; the drain script doesn't know to skip them — see "Skipping previously-blocked tasks" below).

### Skipping previously-blocked tasks

If you want the drain to skip tasks that were blocked in a prior invocation today, filter them out before picking:

```sh
# Get the seq of the next eligible task that has NOT been blocked in the last 24h
NEXT=$(./backlog --profile default task list --project "$PROJECT" --status todo --sort priority --json \
  | jq -r '
    .tasks
    | map(select(
        (.comments // []) | map(.body) | any(test("Blocked after .* attempt"))
        | not
      ))
    | .[0].seq
  ')

if [ -n "$NEXT" ] && [ "$NEXT" != "null" ]; then
  claude -p "/backlog-loop $PROJECT TASK-$NEXT" --permission-mode acceptEdits --max-turns 80
fi
```

This requires comments to be loaded with `task list` — verify the JSON shape matches what `task list` returns. If comments aren't in the listing, use `task show` per candidate.

### Cron example

Drain a project every hour, capped at 5 tasks per run, logged to a rotating file:

```cron
0 * * * * cd /home/me/code/myrepo && /home/me/bin/drain-backlog.sh myproject 5 >> /var/log/backlog-loop.log 2>&1
```

Where `drain-backlog.sh` is the drain script above, with `MAX_INVOCATIONS` parameterized.

### GitHub Actions example

Run the loop on a schedule against a repo that has its `backlog.db` committed (or restored from artifact / Postgres / S3 — your choice):

```yaml
name: backlog-loop
on:
  schedule:
    - cron: "0 */2 * * *"   # every 2 hours
  workflow_dispatch:

jobs:
  drain:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v5
        with: { go-version: "1.25" }
      - name: Build backlog
        run: make build
      - name: Install Claude Code
        run: npm install -g @anthropic-ai/claude-code
      - name: Drain one task
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: |
          claude -p "/backlog-loop myproject" \
            --permission-mode acceptEdits \
            --output-format text \
            --max-turns 80
      - name: Commit backlog.db changes
        run: |
          git config user.name "backlog-loop"
          git config user.email "actions@github.com"
          git add backlog.db || true
          git diff --staged --quiet || git commit -m "chore: backlog-loop run $(date -u +%FT%TZ)"
          git push
```

Caveats:
- Committing `backlog.db` to git works for low-write workloads. For higher write rates, store the DB outside the repo (S3, EFS, a managed Postgres mirror, etc.) and restore/sync in a step.
- Replace `acceptEdits` with a tighter `--allowed-tools` list once you've watched a few runs.
- For private repos, the `GITHUB_TOKEN` permissions matter; review them.

### Useful environment variables

| Var | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Auth for the Claude API. Required unless `claude /login` was run on the host. |
| `BACKLOG_DB` | Absolute path to `backlog.db`. Bypasses profile resolution — recommended for cron/CI. |
| `BACKLOG_PROFILE` | Not honored — the skill uses `--profile default` by default in this repo. Override the skill's flag if you need a different profile. |
| `CLAUDE_CODE_DEFAULT_MODEL` | Default model for the `claude` CLI when `--model` isn't passed. |

### Observability after a run

Every invocation leaves a paper trail in the backlog DB. Read it without re-invoking Claude:

```sh
# Last 20 events on this project
./backlog --profile default activity --project <project> --limit 20

# Recent work by the AI specifically
./backlog --profile default task list --project <project> --actor-kind ai --json

# Tasks blocked in the last day (read their final comments for the diagnosis)
./backlog --profile default task list --project <project> --status todo --json \
  | jq '.tasks[] | select((.comments // [])[] .body | test("Blocked after"))'

# Web UI for browsing
./backlog --profile default web --port 8080
```

### Safety notes for headless runs

- **The skill never marks a task `done` without a Judge PASS.** That guarantee holds in headless mode too — that's the whole point of the Judge gate.
- **Per-task max attempts is 5.** Total cost per invocation is bounded by `--max-turns × per-turn cost`. Watch the first few cron runs to calibrate.
- **The Judge runs as a sub-agent**, which is a separate `Agent` tool call. In headless mode it inherits the same permission mode. Confirm `--permission-mode` isn't too permissive for the verification commands the Judge will run.
- **If the task description is unjudgeable** (no acceptance criteria, no verification commands), the skill refuses to pick it up and exits with a comment suggesting `/backlog-enhance-tasks`. That's a safe headless behavior — it won't silently work on a vague task.
- **Backups before scaling up:** `./backlog --profile default doctor backup --to /safe/backlog.before-loop.db` before the first big drain.

## Quick command reference

```sh
./backlog --profile default project list --json
./backlog --profile default task list --project <project> --status todo --sort priority --json
./backlog --profile default task show TASK-N --json
./backlog --profile default task move TASK-N --status doing --as ai:claude-opus-4-7
./backlog --profile default task move TASK-N --status done  --as ai:claude-opus-4-7
./backlog --profile default task move TASK-N --status todo  --as ai:claude-opus-4-7
./backlog --profile default plan add --task TASK-N --title "..." --content "..." --as ai:claude-opus-4-7 --json
./backlog --profile default comment add "..." --task TASK-N --as ai:claude-opus-4-7
./backlog --profile default task add --project <project> --title "..." --description "..." --as ai:claude-opus-4-7 --json
```

See `skills/backlog/skill.md` for the full CLI surface.
