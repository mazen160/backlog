---
name: backlog-goal
description: Goal-driven autonomous workflow on top of the backlog CLI. Two strict modes — PREP (exhaustive intake, classify the goal shape, decompose into checkpoints + Scout/Judge/Worker tasks, seed the backlog project, stop) and RUN (execute the board, one active task at a time, structured receipts as comments, Worker tasks bounded by allowed_files/verify/stop_if, every checkpoint gated by a spec-driven Judge sub-agent, mandatory final audit before the goal can be marked done). Use when the user says "/backlog-goal <goal>", "/backlog-goal prep <goal>", "/backlog-goal run <slug>", "set a goal end-to-end", or wants a resumable, observable loop that takes a stated goal from blank-page to verified completion through the backlog.
---

# backlog-goal

End-to-end goal pursuit, recorded in a backlog workspace. Every step the agent takes is observable through the `backlog` CLI: tasks for the work, plans for the how, structured-receipt comments for the audit trail, memory for the brief, and a Judge sub-agent that gates every checkpoint.

## Requires: the `backlog` skill

**This skill is a thin orchestrator on top of the `backlog` CLI. Before doing anything in PREP or RUN, load the `backlog` skill** (`skills/backlog/skill.md` in this repo, also exposed as the `backlog` skill in the user's available-skills list). That skill is the canonical reference for:

- every CLI flag (`--profile`, `--as`, `--json`, `--db`, `--quiet`)
- the full surface for `project`, `task`, `plan`, `comment`, `label`, `memory`, `doc`, `attachment`, `import-findings`, `export`, `activity`
- ID formats (`TASK-N`, bare integer, full ULID)
- JSON response shapes for parsing
- enum values for `type`, `status`, `priority`, `actor.kind`
- DB resolution order and profile semantics
- MCP tool names and signatures (if the goal is being executed via MCP rather than the CLI)

If you find yourself guessing a flag name or a JSON field shape, **stop and re-read `skills/backlog/skill.md`**. This skill (`backlog-goal`) assumes you already know how to drive the CLI; it does not re-document it. The "Quick command reference" section at the bottom is a cheat sheet, not a replacement for the full skill.

When operating in *this* repo, also respect the conventions noted in `docs/CLAUDE/MEMORY.md`: always pass `--profile default` (because `go test` can leave an `e2etest` profile registered), always attribute writes to `--as ai:claude-opus-4-7` (or the current model's exact ID), and remember the binary is `./backlog` at the repo root after `make build`.

## Why two modes

The single most common failure of a "set a goal" loop is to skip discovery and start coding. This skill enforces a hard wall: **PREP** asks questions, classifies the goal, and seeds the board, then **STOPS**. **RUN** executes the board the user just confirmed. The user must explicitly invoke RUN — prep never starts work. This invocation boundary is borrowed from GoalBuddy and is non-negotiable: the questions are the work.

## Invocation

```
/backlog-goal               # alias for prep with no argument — ask the user for the goal
/backlog-goal <goal>        # prep mode, default
/backlog-goal prep <goal>   # explicit prep mode
/backlog-goal run <slug>    # execute the board for an existing prepared goal
/backlog-goal status <slug> # print the current board state, no writes
/backlog-goal pause <slug>  # move all `doing` tasks back to `todo`, stop the loop
/backlog-goal clear <slug>  # archive every task in the project; the brief and plan stay on disk
```

If the user types only `/backlog-goal` with arguments that look like a goal description, treat it as **prep**. Only `/backlog-goal run <slug>` switches to execution.

---

## Glossary

| Term | Meaning |
|---|---|
| **Goal** | The user's top-level objective. One goal → one backlog project. |
| **Brief** | Markdown summary of the answers gathered in PREP. Stored as a backlog **doc** in the project. |
| **Checkpoint** | A major verifiable milestone. Stored as a task titled `checkpoint: <name>` with the `checkpoint` label. Gated by the Judge sub-agent. |
| **Scout task** | Read-only investigation task (`scout` label). Produces evidence / a map / a list of candidates. No code edits. |
| **Worker task** | Bounded implementation task (`worker` label). Description must contain `allowed_files`, `verify`, `stop_if`. Writes only inside `allowed_files`. |
| **Judge task** | Read-only decision/audit task (`judge` label). Used for plan validation, ambiguity resolution, checkpoint gating, and the final audit. |
| **PM** | The main agent loop. The PM is the only thing allowed to select the next active task, change task status, mark checkpoints done, or mark the goal done. Scout / Worker / Judge sub-agents return receipts; they do not move the board. |
| **Receipt** | A structured comment posted on a task when work on it concludes (done OR blocked). Schema below. |
| **Parallel safety** | Multiple tasks may be `doing` at once when the PM can prove they won't collide (see "Parallel execution" below). Default is sequential; parallel is opt-in per dispatch. |

---

## PREP mode

Goal: produce a backlog project + brief + plan + seeded task board that the user has confirmed, then stop. Do not write a single non-PREP-related file or run any verification commands. Do not load the named skills the user mentioned in the goal ("use the design skill"). Those go into Scout/Worker tasks for RUN mode.

### P0. Read input

The text after `/backlog-goal` (and optional `prep`) is `$GOAL_INPUT`. It may be:
- a one-liner
- a paragraph
- a file path → read the file
- empty → ask the user for the goal

### P1. Intake compiler (private)

Privately extract the following fields. Do **not** dump them to the user. They drive the question batches in P2.

- `original_request` — shortest faithful copy of the user's wording
- `interpreted_outcome` — one sentence: what must become true when this is done
- `input_shape` — `vague | specific | existing_plan | recovery | audit | eval`
  - `eval` is its own shape: the goal is "drive a metric to a threshold" (e.g. "optimize prompt until eval score ≥ 0.85"). The judge gate becomes a numeric threshold, not a checklist.
- `domain` — one short noun phrase ("game design", "data pipeline", "internal tool", "writing", "design", "research")
- `audience` — who benefits
- `authority` — `requested | approved | inferred | needs_approval | blocked`
- `proof_type` — `test | demo | artifact | metric | review | source_backed_answer | decision`
- `completion_proof` — the observable signal that the *full* outcome is achieved
- `likely_misfire` — concretely, how could this loop succeed at the wrong thing? ("Built a working CSV import that imports the wrong columns.")
- `what_bad_looks_like` — the *user's* description of failure modes from past work, not the agent's guess. Vincent's lesson: pull domain knowledge of past failures from the user.
- `recurring_blind_spots` — bugs / oversights this kind of work usually has, that the user has been burned by before
- `reference_patterns` — design patterns, example outputs, reference screens to match against
- `anti_patterns` — patterns, libraries, or shapes to explicitly avoid
- `blind_spots` — risks the user has not yet named (agent-surfaced, distinct from `recurring_blind_spots`)
- `existing_plan_facts` — if the user provided steps / files / sequencing, preserve them verbatim

### P2. Diagnostic ladder (interactive)

Ask the user. Use `AskUserQuestion` in batches of up to 4 questions. Aim for 8–20 questions total depending on input shape.

**Rules for asking:**

- For vague / open-ended input: one question per batch (sometimes two), each with 2–4 concrete options + a recommended default. After each answer, briefly reflect what it implies, name one blind spot you now see, and ask the next material question. Do not race ahead.
- For specific input: batch related questions together (up to 4).
- For an existing plan: focus questions on *validating the plan* — ambiguities, gaps, risk areas, verification approach.
- Always offer "Other" implicitly (the AskUserQuestion tool does this).

**Question categories — drop categories that obviously don't apply:**

1. **Outcome shape** — What artifact do you receive when this is done? (File on disk? Running service? Published doc? Demo URL?)
2. **Definition of done** — What is the smallest demo / observation that proves the full outcome is reached?
3. **Audience** — Who is this for? (Self, internal team, public, AI agent, future-self.)
4. **Constraints** — Tech stack, language, framework, time budget, performance budget, libraries to avoid, areas not to touch.
5. **Inputs already available** — Existing code, data, designs, references the user wants reused.
6. **Quality bar** — Prototype / demo-ready / production / regulated.
7. **Verification approach** — What command, test, or human review will be used to verify each checkpoint? If "eyeball it", record that — the Judge will flag unverifiable criteria.
8. **Scope cuts** — What is explicitly out of scope? What would you happily *not* build?
9. **Likely misfire** — Walk the user through your `likely_misfire` field and ask them to refine it. ("If I shipped X that did Y, would you be happy? What would make that the wrong outcome?")
10. **Domain-specific** — Ask the questions that only make sense for this domain. For software: language, framework, persistence, deploy target, auth. For game: genre, engine, platform, art style, single/multiplayer, controls. For document: format, length, audience, references. For research: scope, sources allowed, deliverable shape.
11. **Goal kind confirmation** — Confirm `input_shape` with the user. ("This sounds like an open-ended improvement. Treat it that way, or treat it as a specific build, or as an eval-driven optimization?")
12. **What bad looks like** — Ask the user directly: "From past work like this, what does *bad* look like to you? What kinds of bugs do you keep missing?" Vincent's most load-bearing intake question. Capture verbatim into `what_bad_looks_like` and `recurring_blind_spots`.
13. **Reference & anti-patterns** — Ask: "Are there reference patterns, example outputs, or screens the result should match? Are there patterns, libraries, or shapes to avoid?" Feed answers into `reference_patterns` and `anti_patterns`.

**Stop asking when** the user says "stop, just go", OR every applicable category has at least one concrete answer, OR you can write a brief that you'd hand to a fresh agent and trust them to make the right call without consulting the user again.

### P2b. Agent-generated clarifiers

Before writing the brief, **the agent asks its own questions** based on what it has read so far (the user's words, the code, the brief shape). This is Vincent's most-quoted lesson: "ask the model to ask anything before you start."

Read 3–10 relevant files in the working directory (`README`, top-level config, the directory the goal most likely touches). Based on that and the answers so far, generate **3–5 of your own clarifying questions** — things you genuinely don't know that would change how you'd execute. Surface them via `AskUserQuestion`.

Do not generate ceremonial questions ("should I use TypeScript?" when the user already said yes). Surface only questions whose answers would change the plan.

If you can't think of 3 real questions, say so to the user — "I don't have material clarifiers; ready to write the brief" — and skip this step. Forcing fake questions wastes the user's time.

### P3. Write the brief

Compose the brief and store it as a backlog **doc** (versioned). Also keep a copy on disk for the agent's reference.

```markdown
# Goal: <one sentence>

## Original request
<verbatim user wording>

## Interpreted outcome
<one sentence — what must become true>

## Input shape
`vague | specific | existing_plan | recovery | audit | eval` — chosen because <reason>

## Audience
<who it's for>

## Quality bar
<prototype | demo-ready | production | regulated>

## Definition of done
<the smallest demo that proves the FULL outcome>

## Completion proof (quantified)
<observable signal in numbers, exit codes, or strict equality>
e.g. "all screens visually identical to /reference/*.png within 2px tolerance via Playwright"
e.g. "eval score >= 0.85 on the held-out set"
e.g. "20 distinct issues filed, each with: repro steps, proposed fix, branch URL, log entry in run/"

## Stop rule
<the exact condition that ends the loop, stated in the agent's own voice>
e.g. "STOP when the final-audit Judge returns full_outcome_complete: true AND `make test` exits 0 AND <slug>-demo.gif exists"

## Constraints
- <constraint>

## Inputs available
- <existing thing>

## Reference patterns to match
- <design patterns / example outputs / reference screens>

## Anti-patterns to avoid
- <library, shape, idiom>

## Verification approach
<commands, manual review, both — be specific>

## Out of scope
- <explicit non-goal>

## Likely misfire (agent's framing)
<how this loop could succeed at the wrong thing>

## What bad looks like (user's framing)
<verbatim from user — past failures, bugs they keep missing>

## Recurring blind spots in this kind of work
- <user-supplied>

## Blind spots considered (agent-surfaced)
- <risk or unstated choice>

## Existing plan facts (preserve verbatim)
- <user-provided step or constraint>
- (or: "none")
```

Show the brief to the user inline. **Ask them to confirm or correct.** Iterate until they say "looks right". Then write it to disk *and* to backlog:

```sh
# On disk (for the agent's working copy)
mkdir -p docs/backlog-goal/<slug>
# write docs/backlog-goal/<slug>/brief.md

# In backlog (versioned, durable)
./backlog --profile default doc add \
  --project <slug> --title "Brief" \
  --from-file docs/backlog-goal/<slug>/brief.md \
  --as ai:claude-opus-4-7 --json
```

`<slug>` is a kebab-case slug derived from the goal title (e.g. `2d-platformer`).

### P4. Plan with checkpoints

Decompose the goal into **3–7 checkpoints**. A checkpoint is a milestone where stopping makes sense — work before it is meaningfully complete and verifiable, work after it builds on it.

For each checkpoint:
- Title (imperative, specific)
- 2–5 **acceptance criteria** — each must be a single pass/fail observation
- 1–3 **verification commands** — exact shell commands OR an explicit `manual: <what to inspect>` line
- A short list of tasks under it

**Rules:**

- 3–7 checkpoints. Fewer → the goal is too small for this skill, just do it directly. More → split into a sub-goal first.
- Each acceptance criterion must be verifiable — no "works correctly", no "good UX", no "fun". The Judge will fail vague criteria. Treat that as a feature: it forces sharper criteria in PREP rather than relaxed criteria at audit time.
- Order matters. Earlier checkpoints must not depend on later ones.

**Two PREP gates the agent must apply before showing the plan to the user:**

1. **Quantification gate** — every acceptance criterion must reduce to one of: a numeric threshold ("score ≥ 0.85", "p95 latency < 200ms"), a command exit code ("`make test` exits 0", "`bundle size <= 250 KB`"), a strict equality ("Playwright screenshot diff vs `/ref/login.png` ≤ 2px"), a counted artifact ("20 distinct issues filed under `run/`"), or a strict presence/absence ("file `dist/app.wasm` exists", "no `console.log` in `src/`"). Drop or rewrite anything that doesn't fit. "All tests pass" becomes "`pnpm test` exits 0 and reports ≥ N test cases".
2. **Verifiability gate** — every acceptance criterion must have a runnable verification command, NOT a manual review, unless the user explicitly accepted manual review for that criterion in the brief. If the only verifier is "eyeball it", surface this to the user *now*, with options: (a) replace with a Playwright/screenshot/metric command, (b) accept manual review and pre-commit that the user will personally inspect at audit time, (c) drop the criterion. **Do not pass an "eyeball it" criterion silently into the Judge — the final audit will fail it and you'll have wasted a checkpoint.**

Write `docs/backlog-goal/<slug>/plan.md` with this structure:

```markdown
# Plan: <goal>

## Checkpoint 1: <title>
**Acceptance criteria:**
- [ ] <criterion>

**Verification:**
```
<exact command>
```
or
```
manual: <what to inspect>
```

**Tasks under this checkpoint:**
- (scout) <objective>
- (worker) <objective>
- (judge) <objective>

---

## Checkpoint 2: ...
```

Show the plan to the user, confirm, edit if needed.

### P5. Seed the backlog board

Now write the board.

**P5a. Resolve or create the project:**

```sh
./backlog --profile default project list --json
# if <slug> not present:
./backlog --profile default project add "<goal title>" --alias <slug> --as ai:claude-opus-4-7
```

**P5b. Create labels** (role labels + the checkpoint label):

```sh
./backlog --profile default label create "checkpoint" --project <slug> --color "#f59e0b" --as ai:claude-opus-4-7
./backlog --profile default label create "scout"      --project <slug> --color "#3b82f6" --as ai:claude-opus-4-7
./backlog --profile default label create "worker"     --project <slug> --color "#10b981" --as ai:claude-opus-4-7
./backlog --profile default label create "judge"      --project <slug> --color "#a855f7" --as ai:claude-opus-4-7
./backlog --profile default label create "final-audit" --project <slug> --color "#ef4444" --as ai:claude-opus-4-7
```

**P5c. Seed checkpoint tasks** — one task per checkpoint, label `checkpoint`, description carries the acceptance criteria + verification commands (the Judge reads from there):

```sh
./backlog --profile default task add \
  --project <slug> \
  --title "checkpoint: <checkpoint title>" \
  --description "$(cat <<'EOF'
## Acceptance criteria
- [ ] <criterion>

## Verification commands
```
<exact command>
```

## Notes for the Judge
<anything the judge needs>

## Sub-tasks under this checkpoint
<TASK-N list, filled in after P5d>
EOF
)" \
  --type feature --priority P2 \
  --label checkpoint \
  --as ai:claude-opus-4-7 --json
```

Capture each checkpoint's `TASK-N`.

**P5d. Seed the per-checkpoint tasks** — pick a seed shape based on `input_shape`:

| input_shape | First active task type | Why |
|---|---|---|
| `vague` / `open_ended` | scout | Map the space before committing to a slice |
| `specific` + incomplete evidence | scout, then judge | Cheap to confirm before writing |
| `specific` + clear evidence | worker (small) | Just do it |
| `existing_plan` | judge (plan validation) | Preserve plan; check it before executing |
| `recovery` | scout (evidence map) | Don't start from vibes |
| `audit` | judge (read-only audit) | Stay read-only unless approved |

For each task, write a description that a fresh agent can act on with no extra context. Use the shape below.

**Scout task description shape:**
```markdown
## Context
<1-2 sentences>

## Objective
<what the Scout should produce — a map, a list of candidates, an evidence pack>

## Inputs to inspect
- <file path or area>

## Expected output
- <bullet>

## Constraints
- Read-only. No edits.
- Prefer file:line evidence over generic claims.

## Receipt format
Post a comment with the receipt schema (see skill: `[SCOUT RECEIPT]`).
```

**Worker task description shape** — required fields are non-negotiable:
```markdown
## Context
<1-2 sentences>

## Objective
<the single concrete change to make>

## allowed_files
- <path>
- <path>

## verify
- `<exact command>` — must exit 0
- `<exact command>` — must exit 0

## stop_if
- Need files outside allowed_files.
- Behavior is ambiguous.
- Verification fails twice.

## Acceptance criteria
- [ ] <criterion>

## Receipt format
Post a comment with `[WORKER RECEIPT]` (schema below).
```

**Judge task description shape:**
```markdown
## Context
<1-2 sentences>

## Objective
<one of: plan validation | ambiguity resolution | per-slice audit | checkpoint gate | final goal audit>

## Inputs
- TASK-N receipt
- file: <path>

## Decision options
- <choice 1>
- <choice 2>

## Receipt format
Post a comment with `[JUDGE RECEIPT]` (schema below). Decision is one of:
approve_next | reject_next | not_complete | complete
```

Create each task:

```sh
./backlog --profile default task add \
  --project <slug> \
  --title "<scout|worker|judge>: <imperative + specific subject>" \
  --description "<the markdown above>" \
  --type <task|feature|bug|chore> \
  --priority <P1-P5> \
  --label <scout|worker|judge> \
  --as ai:claude-opus-4-7 --json
```

**P5e. Final audit task** — every goal ends with a single `final-audit` Judge task:

```sh
./backlog --profile default task add --project <slug> \
  --title "judge: final audit — does the full original outcome hold?" \
  --description "$(cat <<'EOF'
## Objective
Map every checkpoint receipt back to the brief's "Definition of done" and "Completion proof". Decide whether `full_outcome_complete: true` or false.

## Inputs
- Brief (doc title "Brief", latest version)
- Every checkpoint task receipt
- Every Worker receipt's verification output
- Current dirty diff (`git status`, `git diff`)

## Constraints
- Read-only.
- Reject completion if any required Worker task is still queued, doing, or unverified.
- Reject completion if a checkpoint criterion was relaxed at audit time vs. how it was written in PREP.
- Reject completion if the only proof of done is planning or discovery.

## Receipt format
[JUDGE RECEIPT] with decision = `complete` AND `full_outcome_complete: true`, OR `not_complete` with the missing evidence and the next task to queue.
EOF
)" \
  --type chore --priority P1 --label judge --label final-audit \
  --as ai:claude-opus-4-7 --json
```

**P5f. Seed memory** — used for resumption:

```sh
./backlog --profile default memory add \
  "Goal: <one sentence>. Slug: <slug>. Brief: backlog doc 'Brief' in project <slug>. Started <YYYY-MM-DD>. PREP complete; awaiting `/backlog-goal run <slug>`." \
  --project <slug> --tag "goal,prep-complete" --as ai:claude-opus-4-7
```

### P6. Stop

Print to the user:

```
PREP complete for goal "<title>".

- Project: <slug>
- Brief: backlog doc "Brief" (also at docs/backlog-goal/<slug>/brief.md)
- Plan: docs/backlog-goal/<slug>/plan.md
- Checkpoints: <N>
- Tasks seeded: <count>  (scout: X, worker: Y, judge: Z)
- First active task: TASK-N (<type>)

To execute, run:

  /backlog-goal run <slug>

To refine the board, edit the plan or brief and re-run /backlog-goal prep <slug>.
```

**Do not start work.** Do not read implementation files. Do not load named skills the user mentioned. Do not generate assets. Stop and wait for `/backlog-goal run <slug>`.

---

## RUN mode

Execute the board. The PM (this agent's main loop) is the only thing that selects the active task, dispatches sub-agents, and moves the board.

### Continuation invariant (re-read at the top of every iteration)

The PM holds this invariant in mind for the entire run. State it to yourself before every R1 turn:

> Continuing toward the standing goal in `<slug>`. Take the next concrete step. **Do not accept proxy signals as completion by themselves.** Tests passing is not the same as the feature working. Files changed is not the same as behavior changed. A plan written is not the same as a plan implemented. A migration script that ran is not the same as the migrated app behaving identically. Mark a task or checkpoint done only when the Judge's audit shows the objective has actually been achieved and no required work remains. If you believe the full goal is completed, dispatch the final-audit Judge — do not declare victory yourself.

This is the anti-laziness ward. The single biggest failure mode of long-running agent loops is the agent declaring completion too early.

### R0. Resume context

```sh
./backlog --profile default doc list --project <slug> --json     # find the Brief doc id
./backlog --profile default doc show <brief-doc-id> --json
./backlog --profile default task list --project <slug> --json
./backlog --profile default memory list --project <slug> --tag goal --json
./backlog --profile default memory list --project <slug> --tag learning --json
./backlog --profile default memory list --project <slug> --tag checkpoint --json
```

Read the brief. Read the plan. Read **every** `learning` memory entry — these are the cumulative gotchas, patterns, and decisions from prior turns. Read every `checkpoint` memory entry — these are the timeline. Note all checkpoints (label `checkpoint`) and the final-audit task (label `final-audit`).

The agent must not start dispatching sub-agents in R1 without first reading the learnings. A new sub-agent prompt should include a digest of the relevant learnings so the sub-agent doesn't re-hit a gotcha that was already solved earlier in the run.

### R1. Continuation loop

Repeat until the goal is `done` (final audit passed) or the user interrupts.

#### R1a. Survey active tasks

```sh
./backlog --profile default task list --project <slug> --status doing --json
```

- If any tasks are already `doing` and their sub-agents are still in flight (this turn dispatched them, or a previous turn left them mid-run), let them complete and collect receipts before deciding what to pick next. Don't abandon in-flight work.
- If no eligible work remains until they finish (e.g. the next task depends on their receipts), wait for them. Otherwise, the PM can dispatch additional tasks in parallel — see R1b.

#### R1b. Pick the next active task(s)

Eligibility rules:
1. Skip `checkpoint`-labeled tasks; they are only picked up when all their sub-tasks are `done` (then go to R3).
2. Skip the `final-audit` task until every checkpoint is `done` (then go to R4).
3. Among remaining `todo` tasks, prefer the lowest checkpoint number's tasks first. Within a checkpoint: scout → worker → judge order, with priority P1 > P2 > P3 > P4 > P5 as a tiebreaker.
4. **Parallel batch eligibility** — see "Parallel execution" below. The PM may pick multiple eligible tasks in one turn when their write scopes are provably disjoint.

For each task being dispatched this turn, mark it `doing` and comment, then dispatch the sub-agents in parallel (a single message with multiple `Agent` tool calls):

```sh
./backlog --profile default task move TASK-N --status doing --as ai:claude-opus-4-7
./backlog --profile default comment add "Picked up. Dispatching <scout|worker|judge> sub-agent." --task TASK-N --as ai:claude-opus-4-7
```

#### R1c. Attach a plan if the task is non-trivial

A task is non-trivial if it touches more than one file, mixes more than one concern, or its acceptance criteria are not obvious from the description. Worker tasks with >2 files in `allowed_files` are non-trivial by default.

```sh
./backlog --profile default plan add \
  --task TASK-N --title "Implementation plan" \
  --content "$(cat <<'EOF'
## Steps
1. <concrete action>
2. <next action>

## Testing
- <how to verify>

## Risks
- <risk + mitigation>
EOF
)" --as ai:claude-opus-4-7 --json
```

#### R1d. Dispatch the sub-agent

The PM never executes the task itself. It dispatches a sub-agent via the `Agent` tool with a fully self-contained prompt — the sub-agent has no memory of prior work.

Pick `subagent_type` based on the label:

| Label | subagent_type | Effort |
|---|---|---|
| scout | Explore (or general-purpose for analysis-heavy) | low |
| worker | general-purpose | low–medium |
| judge | general-purpose | high — the Judge must be skeptical |

**Common sub-agent prompt skeleton:**

```
You are <SCOUT|WORKER|JUDGE> for backlog task TASK-N in project <slug>.

Read the task: `./backlog --profile default task show TASK-N --json | jq -r '.description'`

<role-specific contract>

<role-specific receipt format — paste the schema verbatim>

Constraints:
- <Scout/Judge: Read-only. No file edits. No git writes. No service starts.>
- <Worker: Edit ONLY files in `allowed_files`. Run every `verify` command. Stop if any stop_if condition is true. Max 2 fix attempts on verification failures.>
- Report your receipt verbatim back to the PM. Do not move the task on the board.
```

**Worker contract additions (paste into the Worker sub-agent prompt):**
```
- Identify board_path, allowed_files, verify, stop_if from the task description. If any are missing, stop and report `result: blocked` with `stopped_because: missing-fields`.
- Edit only files matching allowed_files. Keep the diff minimal and reversible.
- Run every verify command exactly. Capture stdout/stderr and exit code. Two fix attempts max; then stop.
- Do not spawn further agents. Do not create child sub-goals.
- Return only the receipt. Do not mark the task done.
```

**Judge contract additions:**
```
- You are skeptical by default. Lots of files changed is not completion proof.
- Read receipts and named inputs first. Read raw files only when needed to verify a criterion.
- If a criterion is unverifiable as written, FAIL the criterion and explain what the criterion would need to say to be verifiable.
- For the final audit, only return `complete` AND `full_outcome_complete: true` when every brief acceptance signal is mapped to a receipt + a passing verification command.
- Do not implement. Do not edit. Do not pick the next active task.
```

#### R1e. Record the receipt

When the sub-agent returns, post its receipt verbatim as a comment on the task. Use one of the three schemas below.

**[SCOUT RECEIPT]**
```
[SCOUT RECEIPT]
result: done | blocked
summary: <=120 words
evidence:
  - <file:line or path>
facts:
  - <concrete fact>
contradictions:
  - <conflict surfaced>
ambiguity_for_judge:
  - <thing the Judge should resolve>
candidate_next_tasks:
  - <one-line task seed>
commands:
  - `<command>` → exit <code>
```

**[WORKER RECEIPT]**
```
[WORKER RECEIPT]
result: done | blocked
changed_files:
  - <path>
commands:
  - `<verify command>` → exit <code>
  - `<verify command>` → exit <code>
verification_attempts: <1|2>
summary: <=120 words>
stopped_because: <null or one of: stop_if-condition | verification-failed-twice | missing-fields | files-outside-allowed_files | ambiguity>
remaining_blockers:
  - <if any>
needs_judge: true | false
```

**[JUDGE RECEIPT]**
```
[JUDGE RECEIPT]
result: done | blocked
decision: approve_next | reject_next | not_complete | complete
full_outcome_complete: true | false
rationale: <=120 words>
evidence:
  - <file:line, command output, receipt id>
criteria:
  - [PASS|FAIL] <criterion text> — <evidence>
verification_commands:
  - `<command>` → exit <code>
    <relevant output snippet>
judge_observations:
  - <optional: things outside the criteria that a reasonable reviewer would flag>
missing_evidence:
  - <if not_complete>
next_task_seed:
  - <if not_complete: a one-line task suggestion>
```

Post the receipt:

```sh
./backlog --profile default comment add "$(cat <<'EOF'
<paste the receipt block verbatim>
EOF
)" --task TASK-N --as ai:claude-opus-4-7
```

#### R1e2. Extract learnings to memory

After every receipt, scan it for **durable knowledge** that future runs (or future tasks in this run) would benefit from. If anything qualifies, write it as a backlog memory entry tagged `goal,learning`. Be selective — not every receipt produces a learning. Cheap-to-recreate facts (a single-file path, a one-line config tweak) do not. The bar is: *would a fresh agent picking up the next task save time by reading this?*

What qualifies as a learning:

- **Patterns and conventions** the Scout surfaced from the codebase. ("All HTTP handlers in this repo use `internal/web/middleware.go:Wrap` — must wrap new handlers.")
- **Gotchas a Worker hit and fixed.** ("`modernc.org/sqlite` requires `cgo` disabled — `CGO_ENABLED=0` in the test command.")
- **Decisions made under ambiguity** with their rationale. ("Chose to keep the existing `task_seq` instead of adding a per-project seq — preserves backward compatibility with `TASK-N` refs.")
- **Contradictions surfaced** that the Judge resolved.
- **Blockers and their workarounds.**
- **Anti-patterns confirmed by failure** — something tried and rejected, with the receipt's evidence.

How to write a learning:

```sh
./backlog --profile default memory add \
  "<one-line claim>. Evidence: TASK-N receipt + <file:line or command output>. Applies when: <when this rule fires>." \
  --project <slug> --tag "goal,learning" --as ai:claude-opus-4-7
```

Keep each memory entry tight: one claim, evidence, and the trigger condition. Long-form findings stay in the receipt or in a `docs/backlog-goal/<slug>/notes/<task-id>.md` file linked from the receipt.

The next task picked up in R1b should `backlog memory list --project <slug> --tag learning` and read the entries — they are the cumulative knowledge of the run.

#### R1f. Move the task

- `result: done` and not a checkpoint → `task move TASK-N --status done`.
- `result: blocked` → keep the task `todo`, write a `user-notes.md` entry if a human decision is needed, then spawn a follow-up task that *can* proceed (an adjacent safe slice — see R2 "Blocked ≠ stop").
- `result: done` and it's a checkpoint → see R3.
- `result: done` and it's the final-audit → see R4.

#### R1g. Discover new tasks

If a Scout or Worker receipt names follow-up work (a missing dependency, a flaky test, a refactor needed first), the PM creates new tasks **before** picking the next active task. New tasks reference the originating receipt:

```sh
./backlog --profile default task add --project <slug> \
  --title "worker: ..." \
  --description "Spawned by TASK-N (<receipt summary>). <description shape above>" \
  --label worker --as ai:claude-opus-4-7 --json
```

Comment on the originating task:

```sh
./backlog --profile default comment add "Spawned TASK-M from this receipt." --task TASK-N --as ai:claude-opus-4-7
```

#### R1h. Loop

Go to R1a.

### R1i. Parallel execution

Default is sequential. Parallel dispatch is **opt-in per turn** and only when the PM can prove the in-flight tasks won't collide. The PM is the only thing that decides whether to dispatch in parallel.

**Always-safe parallel:**

- **Multiple Scout tasks** — read-only, no risk. Dispatch in parallel freely whenever evidence-gathering is the next step (e.g. mapping three subsystems at once before a Judge picks the first slice).
- **Multiple Judge tasks** that read disjoint inputs — also read-only.
- **A Scout + a Worker** — the Scout reads, the Worker writes inside its `allowed_files`. Safe as long as the Scout doesn't depend on the Worker's *output* (if it does, run sequentially).

**Conditionally safe parallel — Workers in parallel:**

Two or more Worker tasks may be dispatched in the same turn only when ALL of these hold:
- Their `allowed_files` sets are **provably disjoint**: no file appears in two tasks' `allowed_files`, and no entry in one set is a prefix/parent of an entry in another (e.g. `internal/auth/` overlaps `internal/auth/login.go`).
- Their `verify` commands are independent. If both Workers `go test ./...`, that's fine (read-only on the same files). If both Workers `npm install` against the same `package.json`, that's a collision.
- Neither task's `stop_if` says "behavior is ambiguous" or "needs another task's output" — those should run after the dependency.

If you can't easily prove disjointness, run sequentially. Cost of one extra turn is small; cost of two Workers stomping each other's edits is large.

**Never run in parallel:**

- Two checkpoint Judges on the same checkpoint.
- The final-audit Judge alongside anything else — final audit needs a quiescent board.
- A Worker on file F + a Judge that's evaluating file F. The Judge needs a stable artifact.

**Concrete pattern:**

```
Turn 1: Dispatch Scout-A + Scout-B + Scout-C in parallel (3 Agent tool calls in one message)
   → Collect 3 receipts.
Turn 2: PM reviews receipts, picks 2 disjoint Worker slices.
   → Dispatch Worker-1 + Worker-2 in parallel.
   → Collect 2 receipts.
Turn 3: Dispatch the per-slice Judge sub-agents (sequential or parallel as long as inputs are disjoint).
```

**Collision recovery:**

If two parallel Workers' receipts show overlapping `changed_files` despite disjoint `allowed_files` (sub-agent ignored the constraint), treat both as blocked, revert the last commits of the offending files, spawn a Judge task to decide ordering, and re-run sequentially.

---

### R2. Blocked ≠ stop the goal

A blocked task does not block the goal. The PM keeps making safe local progress:

- If a Worker needs files outside `allowed_files`, spawn a **Judge** task to validate widening the scope, or split the Worker into a smaller slice.
- If a Worker hit ambiguity, spawn a **Scout** task to gather evidence or a **Judge** task to decide.
- If a task needs credentials / production access / a destructive op / a policy decision, that's a *specific-task* block. Spawn a `user-notes.md` entry, mark the task `todo` with a comment explaining the block, and pick a different eligible task. Never set the goal itself to "blocked" for missing credentials.
- Only one of these special states blocks the *whole* goal: every remaining eligible task requires user input. In that case, write a single summary comment on the final-audit task and stop, asking the user to unblock.

### R3. Checkpoint gating

When every non-checkpoint task labeled under checkpoint C is `done`:

1. `./backlog --profile default task move <checkpoint TASK-N> --status doing --as ai:claude-opus-4-7`
2. Comment: `"All sub-tasks done. Dispatching Judge for checkpoint gate."`
3. Dispatch a **Judge** sub-agent (subagent_type: general-purpose, high reasoning) with the contract above. The Judge reads the checkpoint task's description (criteria + verification commands), runs every verification command, and produces a `[JUDGE RECEIPT]`.
4. Post the receipt as a comment.
5. **On `decision: approve_next` and all criteria PASS:** `task move --status done`, comment "Checkpoint cleared by Judge.", **write a `goal,checkpoint` memory entry** (see below), loop to R1.
6. **On `decision: reject_next` or any criterion FAIL:** `task move --status todo`. For each failing criterion, create a new follow-up task (worker or scout, as appropriate) whose description quotes the Judge's evidence and states what must change. Comment on the checkpoint linking the new TASK-N list. **Write a `goal,checkpoint-failed` memory entry** capturing which criteria failed, the iteration number, and the new follow-up task IDs.  Loop to R1. If the same criterion fails 3 times in a row, escalate: write to `user-notes.md`, mark the checkpoint `todo`, and stop.

**Memory writes at checkpoint boundaries (mandatory, not optional):**

On PASS:

```sh
./backlog --profile default memory add \
  "Checkpoint <N> cleared: <checkpoint title>. Judge: <task-N> on <YYYY-MM-DD>. Verified by: <verify commands, exit codes>. Key learnings from this checkpoint: <1-3 bullets quoting the most durable learnings, OR 'none beyond logged'>. Next checkpoint: <N+1, title> or 'final audit'." \
  --project <slug> --tag "goal,checkpoint,checkpoint-pass" --as ai:claude-opus-4-7
```

On FAIL:

```sh
./backlog --profile default memory add \
  "Checkpoint <N> rejected (iteration <K>): <checkpoint title>. Failing criteria: <list>. Judge evidence: <short quote>. Follow-up tasks: TASK-X, TASK-Y. Hypothesis for why this failed: <one sentence>." \
  --project <slug> --tag "goal,checkpoint,checkpoint-fail" --as ai:claude-opus-4-7
```

The point of these entries is **cross-run resumability**: if the goal is paused for a day and another agent (or the same agent in a fresh session) resumes, `backlog memory list --project <slug> --tag checkpoint` is the timeline of where the work stands. Combined with the `goal,learning` entries, they give a fresh agent enough context to keep going without re-deriving everything from receipts.

### R4. Final audit

When every checkpoint is `done`:

1. Pick up the `final-audit` task (`task move --status doing`).
2. Dispatch the Judge sub-agent with the final-audit contract from P5e.
3. Receive the `[JUDGE RECEIPT]`.
4. **On `decision: complete` AND `full_outcome_complete: true`:** mark the audit task `done`. Add a memory entry tagged `goal,completed`. Tell the user the goal is reached, with: total tasks created, checkpoints passed, final artifact location, final verification commands and their exit codes.
5. **On `not_complete`:** quote the Judge's `missing_evidence` and `next_task_seed`. Create the suggested follow-up tasks. Mark the audit `todo`. Loop to R1.

**The goal is never "done" until the final-audit Judge says `full_outcome_complete: true`.** Lots of done tasks is not completion.

---

## Status mode

```
/backlog-goal status <slug>
```

Print a read-only summary. No writes.

```sh
./backlog --profile default task list --project <slug> --json | jq '...'
```

Output shape:
```
Goal: <title>
Slug: <slug>
Brief: <doc title>
Active task: TASK-N (<label>) — <title>
Checkpoints:
  [x] Checkpoint 1: <title>  (Judge: approve_next on <date>)
  [ ] Checkpoint 2: <title>  (3/5 sub-tasks done)
  [ ] Checkpoint 3: <title>  (queued)
Final audit: queued
Tasks: todo=<N>, doing=<1|0>, done=<N>
Recent receipts:
  TASK-12 [WORKER RECEIPT] done — added internal/foo.go, all verify passed
  TASK-11 [JUDGE RECEIPT] approve_next — checkpoint 1 cleared
```

---

## Pause mode

```
/backlog-goal pause <slug>
```

Move every `doing` task in the project back to `todo`, write a comment on each explaining the pause, then stop. Used when the user wants to inspect the board or hand off.

```sh
./backlog --profile default task list --project <slug> --status doing --json | jq -r '.tasks[].id' \
  | while read id; do
      ./backlog --profile default task move "$id" --status todo --as ai:claude-opus-4-7
      ./backlog --profile default comment add "Paused by /backlog-goal pause. Resume with /backlog-goal run <slug>." --task "$id" --as ai:claude-opus-4-7
    done
```

Print: "Paused. N tasks moved back to todo. Resume with `/backlog-goal run <slug>`."

---

## Clear mode

```
/backlog-goal clear <slug>
```

**Destructive — confirm with the user before running.** Archives every task in the project (soft delete). The brief, plan, and memory entries stay; on-disk files in `docs/backlog-goal/<slug>/` stay. The project itself is not deleted.

Before running, prompt the user:

> Clear will archive every task in `<slug>` (N tasks). The brief, plan, and memory entries stay. Continue? (yes/no)

If yes:
```sh
./backlog --profile default task list --project <slug> --include-archived=false --json | jq -r '.tasks[].id' \
  | while read id; do
      ./backlog --profile default task archive "$id"
    done
./backlog --profile default memory add "Cleared on <date>. Brief and plan preserved." --project <slug> --tag "goal,cleared" --as ai:claude-opus-4-7
```

---

## The Judge — design notes

The Judge is the single most important sub-agent. A weak Judge lets the PM declare victory on a broken thing. Rules the Judge must follow (these go into every Judge sub-agent prompt verbatim):

- **Spec-driven** — the only source of truth is the task description's `Acceptance criteria` + `verify` commands + the brief. The Judge cannot invent new criteria or relax existing ones.
- **Read-only** — no file edits. Runs only commands. This separation keeps the Judge honest.
- **Evidence-based** — every PASS/FAIL on a criterion must cite a file:line, a command's stdout/exit code, or a quoted receipt. "Looks good to me" is not acceptable.
- **Skeptical default** — if a criterion is vague or unverifiable, FAIL it and demand a sharper criterion. Don't paper over weak criteria.
- **Reject proxy signals** — paste this list verbatim into every Judge prompt. Do not accept any of these as completion on their own:
  - "Tests pass" → ask: does *the test that was added* actually exercise the acceptance criterion? Run it with `-v`; read the assertions.
  - "Files changed / code written" → behavior is what counts. Run the verify command.
  - "Plan written / design done" → not implementation. A plan does not satisfy a Worker checkpoint.
  - "Migration script ran successfully" → does the migrated *app* behave identically? Spot-check the user-visible surface.
  - "Build succeeded" → does it run and produce the expected output?
  - "Lint clean / typecheck passes" → orthogonal to the requirement.
  - "Sub-agent reports done" → the sub-agent's receipt is input, not a verdict. Verify the claimed evidence independently.
- **Bounded scope** — the Judge evaluates only the named task's criteria. Things outside go into `judge_observations`, not into the verdict.
- **Format-strict** — the `[JUDGE RECEIPT]` schema is parsed downstream. Deviations get rejected by the PM, which re-dispatches the Judge with a "respect the schema" reminder.
- **Final-audit Judge is stricter still** — only returns `full_outcome_complete: true` when every brief signal is mapped to a receipt + passing verification command. The final-audit Judge re-reads the brief's "Definition of done", "Completion proof", and "What bad looks like" and maps each to receipts. If "what bad looks like" has not been actively defended against, FAIL.

---

## Computed gate (do not store as a manual boolean)

The current allowed operations are derived from the active task's label, not stored:

| Active task label | Edits allowed? | Allowed write paths |
|---|---|---|
| `scout` | no | none |
| `judge` | no | none |
| `worker` | yes | only files in `allowed_files` |
| `checkpoint` | no (Judge is running) | none |
| `final-audit` | no (Judge is running) | none |
| (none — between tasks) | PM control files only | `docs/backlog-goal/<slug>/*` |

The PM enforces this when dispatching sub-agents.

---

## Memory tag reference

Every meaningful state transition and durable insight writes a backlog memory entry. The tag taxonomy:

| Tag combination | Written when | Purpose |
|---|---|---|
| `goal,prep-complete` | PREP P5f, after the board is seeded | Resume marker. "PREP is done; awaiting `/backlog-goal run <slug>`." |
| `goal,learning` | RUN R1e2, when a receipt surfaces durable knowledge | The agent's working memory across turns. Read at R0 and before dispatching each sub-agent. |
| `goal,checkpoint,checkpoint-pass` | RUN R3, when the Judge approves a checkpoint | Timeline of which milestones cleared, when, and by what verification. |
| `goal,checkpoint,checkpoint-fail` | RUN R3, when the Judge rejects a checkpoint | Record of failed criteria + hypothesis for why + the follow-up task IDs spawned. |
| `goal,blocker` | RUN R2, when a task is blocked needing user input | What's blocked, what unblocks it, which task. Mirrors `user-notes.md`. |
| `goal,decision` | When the PM (or a Judge) makes a non-obvious design call | The decision, the alternatives considered, why. |
| `goal,completed` | RUN R4, after the final audit passes | "Goal closed on <date>. Final artifact: <path>." |
| `goal,cleared` | Clear mode | "Cleared on <date>. Brief and plan preserved." |

**Rules of thumb:**

- Memory entries are tight: one claim, evidence, when-it-applies. Long-form context goes into receipts or `docs/backlog-goal/<slug>/notes/`.
- Mandatory writes: `prep-complete` (once), `checkpoint-pass` / `checkpoint-fail` (once per Judge verdict), `completed` (once). Skipping any of these breaks resumability.
- Discretionary writes: `learning`, `decision`, `blocker`. Write when the bar is met (would a fresh agent benefit?). Don't spam.
- Memory is the **inter-turn carrier**. Receipts are per-task; comments are per-task; the activity log is append-only audit. Memory is the only place where cross-task, cross-checkpoint knowledge lives.

Read memory entries at R0, and again before dispatching each sub-agent. Pass the relevant learnings into the sub-agent prompt so each sub-agent benefits from the run's accumulated knowledge.

---

## File layout this skill produces

```
docs/backlog-goal/<slug>/
  brief.md           # PREP P3 — answers to the discovery questions (also stored as a backlog doc)
  plan.md            # PREP P4 — checkpoints + tasks
  user-notes.md      # RUN — blockers, Judge escalations, decisions awaiting the user
```

Plus, in the backlog workspace (one project per goal):
- A doc titled "Brief" (versioned)
- Labels: `checkpoint`, `scout`, `worker`, `judge`, `final-audit`
- Tasks: one per checkpoint + Scout/Worker/Judge tasks + one final-audit task
- Plans attached to non-trivial tasks
- Receipt comments (`[SCOUT RECEIPT]` / `[WORKER RECEIPT]` / `[JUDGE RECEIPT]`) on every executed task
- Memory entries tagged `goal,prep-complete`, `goal,completed`, etc.
- Activity log — free, `backlog activity --project <slug>`

---

## Operating rules (hard)

- Always pass `--profile default` (the project memory in `docs/CLAUDE/MEMORY.md` notes that `go test` can pollute the profile registry).
- Always pass `--as ai:claude-opus-4-7` on every write.
- Always pass `--json` when piping output to `jq`.
- **Parallel dispatch is opt-in and PM-controlled.** Sequential by default. Multiple Scouts/Judges may run freely (read-only). Multiple Workers may run only when their `allowed_files` sets are provably disjoint and verify commands don't collide — see R1i. Two checkpoint Judges on the same checkpoint never run in parallel, and the final-audit Judge never runs alongside anything.
- **The PM never executes a task itself.** All meaningful work is dispatched to a sub-agent. Trivial single-line PM file edits to `docs/backlog-goal/<slug>/*` are allowed.
- **Never bypass the Judge.** Checkpoints and the final audit only move to `done` after a `[JUDGE RECEIPT]` says so.
- **Brief and plan are versioned.** Updates to either re-create the backlog doc (new version) and overwrite the on-disk copy. Never lose the prior version.
- **If the user interrupts mid-run**, answer their question, then resume from R0 — the backlog state is the source of truth for resumption.
- **Do not edit files outside `docs/backlog-goal/<slug>/`** without a Worker sub-agent and a receipt.

---

## Quick command reference

```sh
# State
./backlog --profile default project list --json
./backlog --profile default task list --project <slug> --json
./backlog --profile default task show TASK-N --json
./backlog --profile default activity --project <slug> --limit 20
./backlog --profile default doc list --project <slug> --json
./backlog --profile default memory list --project <slug> --tag goal --json

# Create
./backlog --profile default project add "<name>" --alias <slug> --as ai:claude-opus-4-7
./backlog --profile default label create "<name>" --project <slug> --color "#..." --as ai:claude-opus-4-7
./backlog --profile default task add --project <slug> --title "..." --description "..." --label <role> --as ai:claude-opus-4-7 --json
./backlog --profile default plan add --task TASK-N --title "..." --content "..." --as ai:claude-opus-4-7 --json
./backlog --profile default comment add "..." --task TASK-N --as ai:claude-opus-4-7
./backlog --profile default memory add "..." --project <slug> --tag "goal" --as ai:claude-opus-4-7
./backlog --profile default doc add --project <slug> --title "Brief" --from-file docs/backlog-goal/<slug>/brief.md --as ai:claude-opus-4-7
./backlog --profile default doc update <doc-id> --title "Brief" --from-file docs/backlog-goal/<slug>/brief.md --change-note "..." --as ai:claude-opus-4-7

# Move
./backlog --profile default task move TASK-N --status doing --as ai:claude-opus-4-7
./backlog --profile default task move TASK-N --status done  --as ai:claude-opus-4-7
./backlog --profile default task move TASK-N --status todo  --as ai:claude-opus-4-7
```

See `skills/backlog/skill.md` for the full CLI reference.
