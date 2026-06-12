![Backlog — a local-first task queue your AI coding agents read and write](../../assets/banner-blog-release.png)

# Backlog v1.0.3: you can't watch four agents at once

The first version of Backlog solved a state problem. AI coding agents lose everything when a chat ends, so I moved the queue out of the chat and into a local SQLite file that any agent can read and write through MCP. Pick a task, plan it, ship it, attribute the write, exit. Spawn a fresh subagent and do it again.

That loop works. The problem it created is the one this release is about.

<div align="center">
<img src="../../assets/backlog-agentic-loop-editorial.png" alt="The agentic loop: pick, plan, ship, review, attribute, repeat" width="460">
</div>

## An empty queue is not a status report

When one agent is running, you read its output and you know whether the work is any good. When four sessions are clearing the same queue in parallel — which is the whole point — you stop being able to do that. The backlog drains, tasks flip to `done`, and you have no idea whether they were closed cleanly or closed badly.

And agents close work badly in specific, repeatable ways. A task gets marked `done` with no plan attached and no comment explaining what happened. A "final audit" task gets closed while half the project it was auditing is still open. A `doing` task gets abandoned mid-session and sits there for a week looking like active work. None of that shows up in a task count.

So v1.0.3 adds two read-only reports. Neither writes to your database. Both take `--json`.

## `backlog activity analyze`

```sh
backlog activity analyze --project app --since 7d
```

Workflow health for a project over a window: throughput (created vs. completed), cycle time broken down by task type, how long work waits in `todo` before someone picks it up and in `doing` before it ships, work-in-progress by actor, reopened tasks, bug follow-ups, label churn, and the ratio of tasks closed by humans vs. agents.

`--since` takes `7d`, `24h`, `all`, an RFC3339 timestamp, or a plain `YYYY-MM-DD`. It's the report I open first thing to see how the queue moved overnight.

## `backlog doctor project`

```sh
backlog doctor project --project app
```

`backlog doctor check` already verifies the database — integrity check, atomic backup. `doctor project` verifies the *work*. It's a linter for the queue:

- tasks created but never started,
- `doing` tasks gone quiet past `--stale-after` (default a week),
- tasks missing a plan,
- tasks closed with no completion comment or evidence,
- tasks whose only recent activity is a label change,
- final-audit tasks marked done while earlier work is still open.

Every issue carries a severity, a machine-readable code, and the evidence behind it. That last part matters more than it sounds: it means the next subagent can read the report and fix the problem directly, instead of re-deriving what went wrong three sessions later.

## Getting it

```sh
go install github.com/mazen160/backlog/cmd/backlog@latest
```

Still one binary, local-first, no SaaS, no telemetry, MIT. Full notes in the [changelog](https://github.com/mazen160/backlog/blob/main/CHANGELOG.md).

If you're already running agents against a Backlog queue, run `activity analyze` on it once. The first report usually finds something.
