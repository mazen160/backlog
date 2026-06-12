# X / Twitter thread — Backlog launch

**1/**
I'm releasing Backlog: a local-first task queue your AI coding agents read and write directly. One Go binary, one SQLite file, no server.

I built it because my agents kept forgetting the project.

**2/**
An AI coding agent loses its state the second a chat ends. The "memory" everyone leans on is a 500k-token thread that costs you a subscription to keep alive and forgets the project by the next morning.

**3/**
It's worse with more than one. Start a task in Claude Code, continue in Cursor, run one overnight on a box. None of them have heard of the others' work. The plan one wrote is stuck in a chat the next can't read.

**4/**
Backlog moves the queue out of the chat into a local DB every agent reads and writes over MCP, the same file your shell uses. Every write is signed: human:mazin, ai:claude-code, ai:semgrep.

**5/**
Each task spawns a fresh subagent with only what it needs: the task, its plan, the memory. A long thread drifts toward 500k tokens; a focused session fits under 50k. Roughly 10x cheaper per task, about 12x the loops in a day.

**6/**
One binary, MIT, no telemetry, no account.

go install github.com/mazen160/backlog/cmd/backlog@latest

Code and docs: github.com/mazen160/backlog
