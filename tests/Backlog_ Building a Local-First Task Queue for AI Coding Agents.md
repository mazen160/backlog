# Backlog: Building a Local-First Task Queue for AI Coding Agents

May 13, 2026 · 9 mins

## 

If you have spent any real time pairing with Claude Code, Cursor, Codex, or any other AI coding agent, you have probably hit the same wall I did.

You start a task in the evening. The agent reads the codebase, writes a plan, asks a clarifying question, gets it right, ships half the work, comments on the PR. You are tired. You close the tab.

The next morning you open the same agent, type "where were we?", and it has no idea. It can read the code on disk. It can read your open editor tabs. But the actual work state \- the plan, the comments, the decisions, the "we decided not to use Redis because of X" \- is gone. It lived in the conversation, and the conversation is gone.

This gets worse, not better, when you switch IDEs. The agent in Claude Code last night does not hand off to the agent in Cursor this morning. They are not the same agent and they do not share memory. You are the handoff. You can re-use the same session across the development days, but it will compact the context once it hits the context limit \- and that would destroy some of the context and patterns you have worked on.

That’s not the only problem, you can not practically share sessions between Claude, Cursor, Codex, OpenCode today. At best, you’d have to store session IDs in a text file and reference it whenever you’re getting back to tackling a certain problem.

I built [Backlog](https://github.com/mazen160/backlog) because these problems kept slowing me down. It is a local-first task queue your AI coding agents can read and write directly, designed so the work survives the session.

This post is about why it exists, the engineering decisions that shaped it, and what I have learned from a year of running agents against my own backlog.

## What is this post about?

I will break down the problem AI coding agents have with memory, the engineering choices I made building Backlog v1.0, and the patterns I have settled on for running agentic loops in production on real work.

If you are an engineer who uses AI agents daily and has been hand-rolling a backlog out of issue trackers, scratch notes, or memory files, this is the post I wish I had read a year ago.

## The problem: agents forget, humans repeat

The default working model for AI coding agents today is "one long chat thread". Everything the agent knows about your project lives in the active context window. Your plans, your past decisions, the half-finished refactor from yesterday, the bug you decided to defer \- all of it is conversational state.

Two things happen as that conversation grows.

**You hit the context limit, or pay more to extend it.** A serious project session can easily burn 300-500k tokens by lunch. You are paying the model to re-read the same files over and over because nothing was ever written down where the next session could find it. Clearing the session would simply mean losing context. Not only is it expensive, it’s also slow.

**You start tracking memory and tasks in separate markdown files.** Keeping track of progress in Markdown files is a good start, but it doesn’t scale and cause a burden once the project grows.

Both problems get worse when more than one agent is involved. The moment you open Claude Code in one tab and Cursor in another, neither one knows what the other one is doing. They both pick up the same task. They both write the same plan. They both submit overlapping PRs.

And they get worse again when the gap between sessions is long. A task you started in October, deferred, and came back to in January is the worst case: the agent has no idea what was already tried, no idea what was already rejected, no idea what context the previous you assumed. So it starts over.

The fix is not better prompts. The fix is moving the work state out of the conversation and into a place every future session can read from.

## What Backlog does

Backlog is a single Go binary that ships a SQLite-backed task queue with an MCP server built in.

You run `backlog init` in a project directory. That creates a local Backlog database and a config file. Then you point every AI agent you use \- Claude Code, Cursor, Codex, OpenCode \- to the same database via MCP.

From that point on, every task, plan, comment, label, memory entry, doc, and attachment your agents create lives in the same database. Every row carries a typed actor column \- `human:alice`, `ai:claude-code`, `ai:semgrep` \- so you can always answer "who created this?" without guessing.

That file is what makes the work survive between sessions. Open Claude Code tomorrow morning, the same database is there. Switch to Cursor on Wednesday, the same database is there. The agent reads its queue, its plans, its memory entries, and picks up where the previous session left off.

You can also write and read memory notes and lessons learned of a project directly through Backlog. 

The result is the agentic loop:

1. Pick a task from the backlog  
2. Plan the work (versioned, immutable)  
3. Ship the change  
4. Review it, comment on it, attribute it  
5. Pick the next task

That is it. One queue. Many agents. Every write is attributed \- and you can track memory as the project grows.

## What survives between sessions

This is the part I want to spend the most time on, because it is the whole reason Backlog exists.

When the chat ends, five things are normally lost: the memory, the task description, the plan, the comments, and the decisions the agent made along the way. With Backlog, all five live in the same local database. The next session \- same agent or different agent, same IDE or different IDE, same hour or three weeks later \- query Backlog database and sees exactly what the previous session saw.

Let me make this concrete.

**Tasks.** A task is a row with a title, a description, a type, a status, a priority, and an actor. When an agent picks up TASK-12, it does not need to be told what TASK-12 is. It runs `backlog task show TASK-12` and gets the full state.

**Plans.** A plan is a markdown attached to a task. Every edit creates a new immutable version. When the agent reads the task, it reads the current plan. When the next session needs the history, it reads the backlog `plan history <plan-id>` and gets every version, with the actor and timestamp on each one. "Why did we change this?" is answered by reading the change notes on plan v2, plan v3, plan v4. The agent's reasoning across sessions is preserved as a versioned document, not as a chat scrollback that disappears.

**Comments.** Comments are append-only, actor-attributed notes on a task. Agents write completion comments when they ship something ("Verified fix in PR \#142, regression test added in `auth_test.go:284`"). Humans write review comments. The thread on each task is the conversation that survives.

**Memory.** This is the part where I see the most impact coming from.

A memory entry is a tagged free-form note attached to a project. Decisions, assumptions, design rationale, the "we tried X and it did not work" insights. When an agent finishes a task and learns something durable, it writes a memory entry with appropriate tags. When the next session starts up, the first thing it does is `backlog memory list --project <alias> --tag <relevant-tag>` and reads what previous sessions decided.

This is different from a plan, which is scoped to a single task. Memory is project-wide. It is how "we use JWT TTL of 15 minutes because of the auth incident in March" gets preserved as a fact the next agent will consult before it touches anything auth-related.

In practice this looks like:

\`\`\`  
backlog memory add \\

  \--project api \\

  \--body "Auth team decision: JWT TTL \= 15min. Driven by 2026-03 incident where stale tokens were replayed against the v2 endpoint." \\

  \--tag "decision,auth,security" \\

  \--as ai:claude-code  
\`\`\`

\# Three weeks later, a different agent in a different IDE:

backlog memory list \--project api \--tag auth

Memory is the agent's working scratchpad. It is mutable, tagged, queryable, and it survives every session indefinitely.

**Activity log.** Every state transition writes an append-only row: task created, status changed, plan version added, comment added, archived, deleted. The activity log is the agent's audit trail. If you ever need to answer "what happened to this task between Tuesday and Friday?", `backlog activity --entity task --entity-id TASK-12` answers it.

Put together: when you open Claude Code tomorrow morning and it loads the Backlog skills, the first three commands it runs are something like:

backlog task list \--status doing \--json

backlog memory list \--project api \--tag decision \--json

backlog activity \--limit 20 \--json

That is the entire onboarding cost. There is no "where were we?" conversation. The agent reads its own queue, its own decisions, its own recent history, and starts working. The session in the morning is structurally identical to the session you had last night \- same database, same rows, same actor names.

The IDE does not matter. The agent does not matter. The model does not matter. What matters is the database that all of them read and write.

## Engineering decisions

I want to talk about the engineering choices because they were the hardest part of the build, and a few of them go against current industry default.

### 1\. Local-first, not cloud

Backlog is a CLI you run on your machine. The "server" is a SQLite file on your disk. There is no account, no API key, no SaaS bill, and no telemetry.

This is intentional. Three reasons:

- **Latency.** An agent that takes 20ms to read its task queue is fine. An agent that takes 4 seconds to round-trip a SaaS API is not. The agent is going to read and write the queue dozens of times per session.  
- **Flexibility:** Agentic data is private by default. You can `git commit` your backlog db to a private Github repo for backups.

### 2\. MCP-Native API

Backlog exposes its operations over the [Model Context Protocol](https://modelcontextprotocol.io/), the standard JSON-RPC protocol Anthropic introduced for connecting AI assistants to external tools.

### 

3\. Agentic SKILLs for flexibility

Core functionalities of Backlog can be used by Agentic Skills (Claude Skills). In the first release, I’ve developed 4 different skills that you can use to build agentic loops.

### 4\. Single binary, no dependencies

The binary is around 17 MB. It runs on macOS, Linux, and Windows, on arm64 and amd64.

### 5\. Actor attribution at the schema level

Every row in the database carries an `actor_kind` (`human` or `ai`) and an `actor_name` column. Every write is signed.

It is a constraint at the schema level.

You can filter:

backlog task list \--actor-kind ai

backlog task list \--actor-name claude-code

You can audit:

backlog activity \--actor-kind ai \--limit 50

When something weird shows up, you can answer "which agent did this?" instantly.

## What I learned from running agentic loops

### The bottleneck is not model intelligence. It is a task structure.

A vague task ("fix the auth bug") will burn 200k tokens of agent context before you get a useful PR. A well-defined task ("Fix the JWT signature validation in `internal/auth/jwt.go:84`. Add a regression test that forges an unsigned token and asserts a 401.") will finish in under 30k tokens, on the first attempt.

The most leveraged thing you can do as a human in this loop is write better tasks. Backlog gives you a place to write them, a place to revise them with full version history, and a skill (`/backlog-enhance-tasks`) that rewrites vague ones into spec-quality ones before an agent picks them up.

### Fresh subagents beat long threads.

A long-running chat thread accumulates dead context: files the agent loaded an hour ago, plans for tasks that are now closed, decisions that were reversed.

A fresh subagent that pulls a single task, plans it, ships it, and exits operates in a clean context window. In my measurements, the same work that costs around 500k tokens in a long thread costs 30-60k tokens across a series of fresh subagents. That is the order-of-magnitude cost reduction in the headline claim, and it is real.  
I have been running multiple agents against the same Backlog database for about a year now. A few observations.

# Designing The web UI is for humans

`backlog web` serves a clean dashboard. I use it as my standup overview. I open it, see what every agent did overnight, and either approve, comment, or pause the task.

## Try it

go install github.com/mazen160/backlog/cmd/backlog@latest

backlog init

backlog install-skills

That installs the binary, creates a workspace, and writes four agentic-loop skills (`backlog`, `backlog-enhance-tasks`, `backlog-loop`, `backlog-goal`) into Claude Code, Cursor, Codex, and OpenCode, whichever ones you have configured on your machine.

The full documentation lives at [https://mazen160.github.io/backlog/](https://mazen160.github.io/backlog/). The source is MIT-licensed on [GitHub](https://github.com/mazen160/backlog).

## What is next

Backlog v1.0 is what I wanted to ship for a v1: a stable, focused, single-binary backlog with first-class agent support and zero SaaS. There is plenty to build on top of it \- shared team mode, scheduled drains, richer dashboards, a hosted variant for people who want one.

For now, I am running it daily. My agents are running against it daily. If you build with AI agents and you have been hand-rolling a backlog out of issue trackers, scratch notes, or memory files, give it a try.

If it saves you 20 minutes a day, that is roughly two weeks a year. That is the bar I built it to clear.

Best Regards,  
Mazin Ahmed  
