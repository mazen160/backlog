# X / Twitter thread — Backlog v1.0.3

**1/**
Backlog v1.0.3 is out.

When you run one AI coding agent, you can watch it. When you run four against the same queue, you can't — and an empty backlog stops meaning the work was done well.

This release adds two reports that tell you what the agents actually did. 🧵

**2/**
`backlog activity analyze --project app --since 7d`

One read-only report:
• throughput (created vs completed)
• cycle time by task type
• how long work sits in todo before pickup, in doing before it ships
• WIP by actor
• human-vs-AI close ratio

No dashboard. `--json` if you want one.

**3/**
`backlog doctor project --project app`

`doctor check` verifies the database. This verifies the *work*. It flags:
• tasks created but never started
• `doing` tasks gone quiet for a week
• tasks closed with no plan or evidence
• "final audit" marked done while earlier work is still open

**4/**
Every issue comes with a severity, a code, and the evidence behind it.

So the next subagent can read the report and fix the problem — instead of rediscovering it the hard way three sessions later.

**5/**
Both reports are read-only. They don't touch your DB. They just answer the two questions you actually have once the loop runs on its own:

is this project on track, and what got closed badly?

**6/**
Still one Go binary. No SaaS, no telemetry, MIT.

go install github.com/mazen160/backlog/cmd/backlog@latest

Repo + changelog: github.com/mazen160/backlog
