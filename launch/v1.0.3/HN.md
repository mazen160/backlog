# Hacker News — Backlog v1.0.3

## Title

Backlog v1.0.3 – workflow-health reports for AI agent task queues

(Link: https://github.com/mazen160/backlog/releases/tag/v1.0.3)

## First comment

Author here. Backlog is a local-first task queue your AI coding agents read and write through MCP — one SQLite file, single Go binary, every write attributed to the actor that made it (`human:alice`, `ai:claude-code`).

The thing that pushed this release: once you have several agent sessions closing tasks against one queue, "the backlog is empty" stops telling you anything useful. The work can be done *and* done badly — tasks closed with no plan, a "final audit" task marked done while half the project is still open, a `doing` task abandoned a week ago.

So v1.0.3 adds two read-only reports:

- `backlog activity analyze` — throughput, cycle time by task type, todo→doing and doing→done latency, WIP by actor, reopened work, and human-vs-AI close ratio over a time window.
- `backlog doctor project` — a linter for the queue: never-started tasks, stale `doing` work, missing plans, closures with no evidence, label-only activity, premature final audits. Each finding has a severity, a code, and the evidence.

Neither writes to the DB. Both take `--json`.

It's MIT, no telemetry, no account. Happy to answer questions about the data model or the MCP integration.
