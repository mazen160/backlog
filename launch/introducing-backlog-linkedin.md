# LinkedIn post — Backlog launch

I'm releasing Backlog: a local-first task queue your AI coding agents read and write directly.

I built it because my agents kept forgetting the project. An AI coding agent loses its state the moment a chat ends. The "memory" everyone leans on is a 500k-token thread that costs a subscription to keep alive and forgets by the next morning. Run more than one agent and it's worse: start a task in Claude Code, pick it up in Cursor, leave one running overnight, and none of them have heard of the others' work.

We spent a decade teaching humans to fit into Jira. I'm not interested in teaching an LLM to do the same. An agent's queue should live where the agent works: in a file it can read and write, not behind an API with auth tokens and rate limits.

So Backlog is one SQLite file. A CLI for you, an MCP server for your agents, the same database underneath. Every write is signed by whoever made it, human or AI. A fresh subagent pulls one task and stays under 50k tokens instead of dragging a 500k-token thread around. Same work, roughly 10x cheaper.

One binary, MIT, no telemetry: github.com/mazen160/backlog

I'd like to hear how it holds up if you're running agents in production.
