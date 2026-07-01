# Backlog Skills

The backlog repo ships five skills for Claude Code, Cursor, Codex, and OpenCode. Skills are markdown files — no code, no binary — that become part of the AI assistant's context when invoked. They teach the assistant how to use the `backlog` CLI and how to layer richer workflows on top of it.

## Skills inventory

| Skill | Purpose | Invocation |
|---|---|---|
| `backlog` | The base skill. Full CLI reference: every command, flag, ID format, enum, JSON shape, common agent workflow. Every other skill depends on this one. | `/backlog <natural-language request>` |
| `backlog-memory` | Project memory in two modes — **learn** (load a project's tasks, plans, docs, and memory into the session) and **store** (synthesize the project's state into persistent memory entries). Auto-selects the mode. | `/backlog-memory [learn\|store] [project]` |
| `backlog-enhance-tasks` | Rewrites a task's title and description for clarity, adds structured sections (Context / Acceptance criteria / Implementation hints), optionally generates an implementation plan. | `/backlog-enhance-tasks TASK-N [--build-plan]` |
| `backlog-loop` | Picks up one task, iterates implementation → verification → Judge sub-agent up to 5 attempts until the Judge approves, then exits. Designed to run headlessly via `claude -p`. | `/backlog-loop <project> [TASK-N]` or `/backlog-loop help` |
| `backlog-goal` | Goal-driven autonomous workflow. Two strict modes: PREP (exhaustive intake, classify the goal, decompose into checkpoints, seed the board, stop) and RUN (execute the board with Scout/Judge/Worker sub-agents, parallel-safe, every checkpoint gated by a Judge, final audit before done). | `/backlog-goal <goal>` then `/backlog-goal run <slug>` |

Each skill declares its dependency on the `backlog` skill at the top of its file, so an agent loading `backlog-goal` (for example) is reminded to read `backlog/skill.md` first for CLI semantics.

## Installation

### Automatic install

All five skills are embedded in the `backlog` binary.

```sh
backlog install-skills
```

Scans `$HOME` for supported AI coding tools and writes every embedded skill into each one in that tool's native format. Re-runs are idempotent (existing files are skipped unless `--force` is passed).

### Targets

| Tool        | Detected by         | Skill file                                         |
|-------------|---------------------|----------------------------------------------------|
| Claude Code | `~/.claude/`        | `~/.claude/skills/<name>/skill.md`                 |
| Cursor      | `~/.cursor/`        | `~/.cursor/rules/<name>.mdc` (with frontmatter)    |
| OpenCode    | `~/.config/opencode/` | `~/.config/opencode/skills/<name>/skill.md`      |
| Codex       | `~/.codex/`         | `~/.codex/skills/<name>/SKILL.md`                  |

### Useful flags

- `--all` — install everywhere even if the tool's config dir is missing.
- `--force` — overwrite existing skill files.
- `--dry-run` — print what would be written.
- `--skill <name>` — install only the named skill (repeatable).

You can also keep the skills at the project level by leaving `skills/` in the repo — Claude Code picks up project-scoped skills automatically when invoked inside the repo.

---

## `backlog` — the base skill

The canonical reference for driving the CLI. Every other skill assumes the agent has loaded this one.

What it covers:

- Workspace, profile, project, task, plan, comment, label, memory, doc, attachment commands
- Global flags: `--profile`, `--db`, `--json`, `--as`, `--quiet`
- ID formats: `TASK-N`, bare integer, full ULID
- DB resolution order: `--db` → `$BACKLOG_DB` → `--profile` → default profile
- Enum values for `type`, `status`, `priority`, `actor.kind`
- JSON response shapes for every list/show command
- Import-findings file format for bulk task creation
- MCP tool surface (when `backlog mcp serve` is running)
- Common agent workflows (triage findings, pick up a task, revise a plan, capture a decision as memory)

### Usage patterns

```
/backlog add a task to fix the login timeout in the auth service
/backlog list all open P1 vulnerabilities in the api project
/backlog show TASK-12 and its plan history
/backlog import the findings from scan-2026-05.json into project api
```

The skill instructs the assistant to translate the natural-language request into the matching CLI invocation, parse `--json` output, and present results in a readable form.

---

## `backlog-memory`

Persistent project memory in two modes. This is the skill that makes Backlog's cross-session "context" half work: a fresh session loads what earlier sessions learned, and a finished session writes new knowledge back.

### Invocation

```
/backlog-memory                  # auto-pick the mode for the default project
/backlog-memory <project>        # auto-pick the mode for a project
/backlog-memory learn [project]  # force learn (read-only)
/backlog-memory store [project]  # force store
```

### Two modes

- **learn** (read-only) — reads a project's tasks, plans, docs, and memory into the current session, so the agent starts with full situational awareness instead of re-deriving what is already known.
- **store** — synthesizes the project's current state into persistent memory entries, grouped by theme, so the next session loads context instantly.

When no mode is given it auto-selects: **learn** at the start of a fresh session, **store** after work has been done this session, and it asks when that is ambiguous.

### When to use

- At the start of a session, to orient on a project without re-reading the whole codebase.
- At the end of a session, to persist decisions, gotchas, and open-work summaries for the next agent.

---

## `backlog-enhance-tasks`

Improves a single task's title, description, and (optionally) attaches an implementation plan.

### Invocation

```
/backlog-enhance-tasks TASK-N              # rewrite title and description
/backlog-enhance-tasks TASK-N --build-plan # plus attach a plan
```

### What it does

1. Fetches the task with `backlog task show TASK-N --json`.
2. Rewrites the title — imperative verb, specific subject, under 80 chars. Does not change scope.
3. Expands the description into structured markdown:

   ```markdown
   ## Context
   <1-2 sentences on why this matters>

   ## Acceptance criteria
   - [ ] <specific, testable criterion>

   ## Implementation hints
   <file paths, function names, only if clearly known>
   ```

4. Writes back with `backlog task update`.
5. If `--build-plan` was passed, generates an implementation plan with numbered steps + testing section and attaches it via `backlog plan add`.

### When to use

- Before running `/backlog-loop` on a task that's too vague to be judgeable.
- Before handing a backlog item to a sub-agent or a human teammate.
- As part of triage, to convert a rough one-liner into something actionable.

---

## `backlog-loop`

Single-task primitive with a built-in Judge gate. Picks one task, iterates implement → verify → judge → fix → re-judge until the Judge approves or 5 attempts have failed, then exits. Never picks up a second task.

### Invocation

```
/backlog-loop <project>          # next highest-priority todo task
/backlog-loop <project> TASK-N   # a specific task
/backlog-loop help               # print the headless-execution guide
```

### Workflow

1. **Select** — first task by `--status todo --sort priority`, or the explicit ref.
2. **Judgeability check** — if the description has no verifiable criteria, refuse to pick up and suggest `/backlog-enhance-tasks` first. A loop without a Judge gate is the failure mode this skill exists to prevent.
3. **Move to doing**, comment "attempt 1 of max 5".
4. **Attach a plan** if non-trivial (more than one file, more than one concern, or non-obvious criteria).
5. **Iterate (max 5 attempts):**
   - Implement (from scratch on attempt 1, from the prior Judge's `next_fix_hint` on attempts 2–5).
   - Run verification commands.
   - Dispatch the Judge sub-agent with a self-contained prompt and a strict `[JUDGE RECEIPT]` schema. The Judge reads the task description, runs every verification command, evaluates each criterion with cited evidence, and rejects proxy signals (tests pass ≠ feature works, files changed ≠ behavior changed, plan written ≠ implemented, build green ≠ correct output, agent's own claim of done).
   - Post the `[JUDGE RECEIPT]` as a comment. PASS → break. FAIL → next attempt.
6. **On PASS** — completion comment with changes/verification/follow-ups, `task move --status done`, print one-line summary, exit.
7. **On 5 failures or hard blocker** — diagnostic comment (what kept failing, what was tried each attempt, what's needed to unblock), `task move --status todo`, exit.

### Headless execution

`backlog-loop` is designed for `claude -p` (non-interactive print mode):

```sh
claude -p "/backlog-loop <project>" \
  --permission-mode acceptEdits \
  --output-format stream-json \
  --max-turns 80
```

Three clean exit states all return code 0: task done, task blocked (with diagnostic comment), or no work. Non-zero exit = harness error, never a task failure. The skill's `help` invocation prints the full headless guide including drain script, cron entry, and a GitHub Actions example.

### When to use

- Clearing one well-scoped item off the backlog with verification.
- As the inner step of a drain script that empties a project task by task.
- As a scheduled cron / CI job that keeps a long backlog moving.

---

## `backlog-goal`

End-to-end goal pursuit. Maps a stated goal to a backlog project, decomposes it into checkpoints with verifiable acceptance criteria, executes via Scout/Judge/Worker sub-agents with a strict completion gate.

### Invocation

```
/backlog-goal               # alias for prep with no argument — ask for the goal
/backlog-goal <goal>        # prep mode (default)
/backlog-goal prep <goal>   # explicit prep mode
/backlog-goal run <slug>    # execute the prepared board
/backlog-goal status <slug> # read-only board summary
/backlog-goal pause <slug>  # move all doing tasks back to todo, stop
/backlog-goal clear <slug>  # archive all tasks; brief and plan preserved
```

### Two strict modes

**PREP** asks, classifies, seeds, **stops**. It never starts work. Phases:

1. **Intake compiler (private)** — extracts `input_shape`, `domain`, `audience`, `authority`, `proof_type`, `completion_proof`, `likely_misfire`, `what_bad_looks_like`, `recurring_blind_spots`, `reference_patterns`, `anti_patterns`, `existing_plan_facts`.
2. **Diagnostic ladder (interactive)** — `AskUserQuestion` in batches across 13 question categories. Vague input gets one question per batch with 2–4 concrete options + recommended default; the agent reflects on each answer before asking the next.
3. **Agent-generated clarifiers** — after the canned categories, the agent reads relevant code and asks 3–5 of its own questions.
4. **Brief** — written to disk *and* stored as a versioned backlog doc titled "Brief".
5. **Plan with 3–7 checkpoints** — each with quantified acceptance criteria + runnable verification commands. Two PREP gates apply: **quantification gate** (every criterion reduces to a number, exit code, strict equality, counted artifact, or presence/absence) and **verifiability gate** (every criterion has a runnable verifier, OR is explicitly accepted as manual review).
6. **Seed the board** — creates the project, role labels (`checkpoint`, `scout`, `worker`, `judge`, `final-audit`), the checkpoint tasks (with criteria and verify commands in the description), the per-checkpoint Scout/Worker/Judge tasks (Workers have `allowed_files`, `verify`, `stop_if`), and the mandatory final-audit Judge task.

**RUN** executes the board. The PM (main loop) is the only thing that selects tasks, dispatches sub-agents, and moves the board. Key rules:

- **Continuation invariant** — the PM re-reads at the top of every iteration: "Do not accept proxy signals as completion. Tests passing ≠ feature working. Files changed ≠ behavior changed. Plan written ≠ implemented. Mark a task done only when the Judge's audit shows the objective has actually been achieved."
- **Sub-agent roles** — Scout (read-only mapper, low effort), Worker (bounded writer, low effort), Judge (read-only decider, high effort). Each returns a structured receipt; only the PM moves the board.
- **Receipt schemas** — `[SCOUT RECEIPT]`, `[WORKER RECEIPT]`, `[JUDGE RECEIPT]` posted as comments. Parseable, evidence-based.
- **Parallel execution** — opt-in per dispatch. Always safe: multiple Scouts/Judges (read-only). Conditionally safe: multiple Workers with provably-disjoint `allowed_files` and non-colliding verify commands. Never parallel: two checkpoint Judges on the same checkpoint, the final-audit Judge alongside anything.
- **Checkpoint gating** — when every sub-task under a checkpoint is done, a Judge sub-agent verifies the checkpoint against its written criteria. PASS → done. FAIL → spawn follow-up tasks quoting the Judge's evidence; 3 consecutive same-criterion fails escalates to the user.
- **Final audit** — single Judge task labeled `final-audit` that maps every checkpoint receipt back to the brief's "Definition of done" and "What bad looks like". The goal is never done until this Judge returns `decision: complete` AND `full_outcome_complete: true`.

### Memory writes

Every meaningful state transition writes a backlog memory entry. The taxonomy:

| Tag | When |
|---|---|
| `goal,prep-complete` | After PREP seeds the board |
| `goal,learning` | After any receipt that surfaces durable knowledge — patterns, gotchas, decisions under ambiguity |
| `goal,checkpoint,checkpoint-pass` | Every Judge approval of a checkpoint |
| `goal,checkpoint,checkpoint-fail` | Every Judge rejection — captures failing criteria, iteration, hypothesis |
| `goal,blocker` | Any task blocked needing user input |
| `goal,decision` | PM/Judge design call worth preserving |
| `goal,completed` | Final audit passes |
| `goal,cleared` | Clear mode |

Memory is the inter-turn carrier. Receipts are per-task; the activity log is per-event. Memory is the only place where cross-task, cross-checkpoint, cross-session knowledge lives. The PM reads `--tag learning` at the start of every RUN turn and passes relevant entries into sub-agent prompts.

### When to use

- Multi-hour or multi-day coding objectives.
- Open-ended improvement goals where you don't yet know the exact slices.
- Migrations and refactors that need staged verification.
- Anything where the agent's own claim of "done" wouldn't be trustworthy.

For one well-scoped item, use `/backlog-loop` instead.

---

## Common conventions across all skills

### Actor attribution

Every write must be attributed:

```
--as ai:claude-opus-4-7
--as ai:claude-sonnet-4-6
--as ai:claude-code
--as human:alice
```

Defaults to `human:$USER` when `--as` is omitted, but agents must pass it explicitly. Filter by actor with `backlog task list --actor-kind ai` or `--actor-name claude-code`.

### Profile flag in this repo

Always pass `--profile default` when running the CLI inside the backlog repo itself. The reason: `go test` can leave an `e2etest` profile registered in `~/.config/backlog/config.toml`, and if it becomes the default profile the agent will silently target the wrong workspace. The project memory at `docs/CLAUDE/MEMORY.md` documents this.

### JSON output for parsing

Always pass `--json` when piping to `jq` or otherwise programmatically parsing.

### Completion protocol

Across these skills, the completion sequence for a task is:

1. Do the work.
2. Post a comment summarizing what was done (Scout/Worker/Judge receipt for the goal/loop skills; plain summary for the base skill).
3. Move the task to `done`:
   ```sh
   backlog task move TASK-N --status done --as "ai:<model-name>"
   ```

`backlog-loop` and `backlog-goal` additionally gate this behind a Judge sub-agent. The base `backlog` skill and `backlog-enhance-tasks` do not.

---

## Skill file locations reference

### Source (in this repo)

```
skills/backlog/skill.md                # base skill
skills/backlog-memory/skill.md         # project memory: learn + store
skills/backlog-enhance-tasks/skill.md  # title + description enhancer
skills/backlog-loop/skill.md           # single-task iterator with Judge gate
skills/backlog-goal/skill.md           # goal-lifecycle with checkpoints
```

### Installed (user-level)

| Tool | Path |
|---|---|
| Claude Code | `~/.claude/skills/<name>/skill.md` |
| Cursor | `~/.cursor/rules/<name>.mdc` |
| OpenCode | `~/.config/opencode/skills/<name>/skill.md` |
| Codex | `~/.codex/skills/<name>/SKILL.md` |

Skills are embedded in the `backlog` binary via `skills/skills.go`. `backlog install-skills` writes them into whichever of the four tool directories exist under `$HOME`.

### Dependency graph

```
backlog-goal    backlog-loop    backlog-enhance-tasks    backlog-memory
     │               │                   │                     │
     └───────────────┴───────────────────┴─────────────────────┘
                                 │
                                 ▼
                          backlog  (canonical CLI reference)
```

Every workflow skill depends on the `backlog` skill for command surface, flag semantics, and JSON shapes. Each workflow skill declares this dependency at the top of its file.
