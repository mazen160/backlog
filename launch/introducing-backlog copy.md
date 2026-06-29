![Backlog - your AI agent shouldn't ask "what's next?" It should just check $ backlog](../website/assets/blog-banner-backlog-intro.png)

# Backlog: A Local-First Task Queue Your AI Coding Agents Read and Write

I'm releasing Backlog, a local-first task queue your AI coding agents read and write directly. One Go binary, one SQLite file next to your code, and your agents talk to it over MCP. No server, no SaaS, no account.

I built it because my agents kept forgetting the project. Here's how I see it.

An AI coding agent loses its state the moment a chat ends. The "memory" everyone leans on is a 500k-token thread that costs you a subscription to keep alive and forgets the project by the next morning. You open a fresh session, the agent re-reads the codebase, re-derives decisions you already made, and asks you what's next.

It gets worse the moment you run more than one. You start a task in Claude Code, pick it up in Cursor, leave one running overnight on a box. None of them have heard of the others' work. The plan one agent wrote is stuck in a conversation the next one can't read.

We've spent two years bolting "memory" onto chatbots. Has it fixed this? Not really. The state still lives in the chat, and the chat disappears.

![One Backlog database at the center, read and written by several AI agents through MCP](../assets/backlog-concept-one-queue-many-agents.png)

*One database at the center. Every agent reads and writes the same queue, and every write is attributed.*

We spent a decade teaching humans to fit into Jira. I'm not interested in teaching an LLM to do the same. An agent's queue should live where the agent works. In a file it can read and write, not behind a web API with auth tokens and rate limits.

So that's what Backlog is. One SQLite file. A CLI for you, an MCP server for your agents, the same database underneath both. When an agent writes a task, you see it in your shell. When you move a task, the agent sees it on its next call.

```sh
backlog init
backlog task add -p app -t "Fix unsigned JWT rejection in auth middleware" \
  --type vulnerability --priority P1
backlog mcp serve --as ai:claude-code
```

From there the loop is simple. An agent pulls the next task, attaches a plan, ships it, leaves a completion comment, marks it done, and grabs the next one. Pick, plan, ship, review, attribute, repeat.

![The agentic loop: pick, plan, ship, review, attribute, repeat](../assets/backlog-agentic-loop-editorial.png)

*One unit of work: pick, plan, ship, review, attribute, then pick the next one.*

Each task spawns a fresh subagent with only what it needs - the task, its plan, the relevant memory. A long-running thread carries everything it has ever seen and drifts toward 500k tokens. A focused session fits under 50k. Same work, roughly 10x cheaper per task, and a developer running fresh sessions across a day closes about 12x the loops of one bloated thread.

My background is security research, so two things mattered to me from the start. The first is attribution. Every task, plan version, comment, and memory entry records the actor that wrote it - human:mazin, ai:claude-code, ai:semgrep. You always know who did what, and you can filter on it. The second is that the queue should take machine input as a first-class citizen. A scanner can write its findings straight in.

```sh
backlog import-findings findings.json --as ai:semgrep
```

Every finding becomes a task, attributed to the tool that found it.

![Backlog's web UI showing a tasks table with mixed human and AI agent attribution](../assets/web-ui.png)

*`backlog web` serves the same database in the browser. Human and agent writes sit side by side, each tagged with who made it.*

Backlog isn't a replacement for Linear or Jira, and I'm not pretending it is. It's local-first and single-writer. No real-time collaboration, no integrations marketplace, no per-seat dashboard for fifty stakeholders. If that's what you need, use the SaaS tool. It's good at that. Backlog is for the other thing: the AI's working queue, and the state that has to survive across sessions, tools, and machines.

```sh
go install github.com/mazen160/backlog/cmd/backlog@latest
backlog init
backlog install-skills
```

One binary, MIT, no telemetry, no account. `install-skills` writes the agentic-loop skills into Claude Code, Cursor, Codex, and OpenCode, so your agents know how to drive the CLI on day one. The code is at github.com/mazen160/backlog.

I'll end with the question I keep coming back to. When your agents close a sprint against one queue overnight, can you tell what they actually did, and who did it? If the answer is no, the state is in the wrong place.

Best Regards,
Mazin



I have been working with agentic AI for years and always struggled with managing the tasks and to‑dos it generates, transferring knowledge between sessions, and improving products without keeping a single session for long. Keeping a session for days builds up context, and when I send a request to Claude, for example, I am charged for the whole request. Compression could help, but it’s still expensive. AI agents forget what has happened, small details, and there’s no way to maintain this.

I started building Backlog, an NGIN Tech AI task and context manager made for humans and AI. Backlog is a local‑first task queue for coding agents that lets you maintain notes, to‑dos, tasks, and project documentation in Markdown—the official language for AgentDKI. It keeps everything organized, tracks status, and prevents messy artifacts. It also stores an activity log for every task, plans, documentation, lessons learned, and memories within the project.

Backlog supports cloud, code, chat‑GBT codecs, a cursor, Pi agent, and any other agent API. Users can view a visual UI of what agentic AI agents are working on and where they are. Agents can leave comments like a human engineer to track changes for each task. For example, when I run a code review on a repository that generates 40 changes, I create a ticket for each change, let the AI agent pick them up, and update the tickets with changes, lessons learned, and everything. At the end of the session, I ask the AI agent to store memory in Backlog, keeping everything consistent. When I start a new session, I can read the memory and immediately learn about the project and lessons learned without re‑reading the whole project.

Backlog’s UI lets me navigate project tickets and their progress. I also developed a skill slash backlog enhanced tasks that can update research items directly within a ticket without human intervention. I use Backlog for the past 30 days on internal projects, and it has significantly boosted productivity and quality of changes. I have also developed four different projects that I will release soon, all powered through Backlog.

Use cases I’ve done: I no longer maintain random documentation artifacts; all artifacts are stored as documents within Backlog. I run regular Renovated Discovery and security reviews on code bases, store findings on Backlog, and have another agent resolve them. I store memory of changes and lessons learned on each project, assign them within the project, and trace them.
