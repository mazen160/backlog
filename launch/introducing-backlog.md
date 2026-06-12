![Backlog — your AI agent shouldn't ask "what's next?" It should just check $ backlog](../website/assets/blog-banner-backlog-intro.png)

# Backlog: a local-first task queue your AI coding agents read and write

I'm releasing Backlog, a local-first task queue your AI coding agents read and write directly. It's a single Go binary, it keeps everything in one SQLite file next to your code, and your agents talk to it over MCP. No server, no SaaS, no account.

I built it because my agents kept forgetting the project.

## The chat has context. The project doesn't.

An AI coding agent loses its state the moment the chat ends. The "memory" everyone leans on is a 500k-token thread that costs you a subscription to keep alive and forgets the project by the next morning. Open a fresh session and the agent re-reads the codebase, re-derives decisions you already made, and asks you what's next.

It gets worse the moment you run more than one. Start a task in Claude Code at your desk, pick it up in Cursor on the couch, leave an agent running on a remote box overnight — none of them have heard of the others' work. The plan one agent wrote is trapped in a conversation the next one can't see.

We spent a decade teaching humans to fit into Jira. I'm not interested in teaching an LLM to do the same. An agent's queue should live where the agent works: in a file it can read and write, not behind a web API with auth tokens and rate limits.

![One Backlog database at the center, read and written by several AI agents through MCP](../assets/backlog-concept-one-queue-many-agents.png)

*One database at the center. Every agent reads and writes the same queue, and every write is attributed.*

## Why I built it

The thing that pushed me was running a few agent sessions against one project and having nowhere for them to coordinate. Each one was smart in isolation and useless as a team. I wanted the queue out of the chat and into a small local database every agent could read and write, with every change signed by whoever made it.

That's all Backlog is. One SQLite file. A CLI for you, an MCP server for your agents, the same database underneath both. When an agent writes a task, you see it in your shell. When you move a task, the agent sees it on its next call.

## How it works

You initialize a workspace and add work the way you'd expect:

```sh
backlog init
backlog task add -p app -t "Fix unsigned JWT rejection in auth middleware" \
  --type vulnerability --priority P1
backlog mcp serve --as ai:claude-code
```

From there the loop is simple: an agent pulls the next task, attaches a plan, ships it, leaves a completion comment, marks it done, and grabs the next one. Pick, plan, ship, review, attribute, repeat. Each task spawns a fresh subagent with only the context it needs: the task, its plan, and the relevant memory.

![The agentic loop: pick, plan, ship, review, attribute, repeat](../assets/backlog-agentic-loop-editorial.png)

*One unit of work: pick, plan, ship, review, attribute, then pick the next one.*

That last part is where the cost goes. A long-running thread carries everything it has ever seen and drifts toward 500k tokens. A fresh subagent working one task off the queue usually fits under 50k. Same work, roughly 10x cheaper per task, and a developer running fresh sessions across a day closes about 12x the loops of one bloated thread.

My background is security research, so two things mattered to me from the start. The first is attribution: every task, plan version, comment, and memory entry records the actor that wrote it: `human:mazin`, `ai:claude-code`, `ai:semgrep`. You always know who did what, and you can filter on it. The second is that the queue should accept machine input as a first-class citizen. A scanner can write its results straight in:

```sh
backlog import-findings findings.json --as ai:semgrep
```

Every finding becomes a task, attributed to the tool that found it. The morning standup writes itself from the activity log.

![Backlog's web UI showing a tasks table with mixed human and AI agent attribution](../assets/web-ui.png)

*`backlog web` serves the same database in the browser. Human and agent writes sit side by side, each tagged with who made it.*

## What it isn't

Backlog isn't a replacement for Linear or Jira, and I'm not pretending it is. It's local-first and single-writer. It has no real-time collaboration, no integrations marketplace, no per-seat dashboard for fifty stakeholders. If that's what you need, use the SaaS tool — it's good at that.

Backlog is for the other thing: the AI's working queue and the state that has to survive across sessions, tools, and machines. It does that one job, and it does it without a server.

## Try it

```sh
go install github.com/mazen160/backlog/cmd/backlog@latest
backlog init
backlog install-skills
```

One binary, no dependencies, no telemetry, MIT. `install-skills` writes the agentic-loop skills into Claude Code, Cursor, Codex, and OpenCode so your agents know how to drive the CLI on day one.

The code is at [github.com/mazen160/backlog](https://github.com/mazen160/backlog). If you're running coding agents in anything close to production, I'd genuinely like to hear how the queue holds up for you, and what breaks.
