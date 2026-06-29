# LinkedIn post — Backlog v1.0.3

Backlog v1.0.3 is out, and it's about a problem you only hit once the agentic loop is working: observability.

When one AI agent is closing tasks, you watch it. When four are running in parallel against the same queue, you can't — and an empty backlog stops being proof that the work was done well. So this release adds two read-only reports that tell you what actually happened.

**`backlog activity analyze`** — workflow health for a project over a time window. Throughput, cycle time by task type, how long work waits before someone picks it up, WIP by actor, and the human-vs-AI close ratio. One command, `--json` if you want to chart it.

**`backlog doctor project`** — a linter for the queue itself. It flags tasks created but never started, `doing` tasks gone quiet for a week, tasks closed with no plan or completion evidence, and "final audit" tasks marked done while earlier work is still open. Each issue carries a severity, a code, and the evidence behind it, so the next agent can fix it from the report alone.

`doctor check` verifies the database. `doctor project` verifies the work. That distinction turned out to matter a lot once agents were closing tasks faster than I could read them.

Still one Go binary. Local-first, no SaaS, no telemetry, MIT.

go install github.com/mazen160/backlog/cmd/backlog@latest

Repo and full changelog in the comments.
