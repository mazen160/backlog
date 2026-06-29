# Backlog v1.0.3

**Workflow observability for agent-driven queues.**

When you run one AI agent, you can watch it. When you run four in parallel against the same queue, you can't — and an empty backlog stops being a reliable signal that the work was actually done well. v1.0.3 adds two read-only reports that answer the two questions that matter once the loop is running on its own: *is this project on track?* and *what did the agents close badly?*

Both run from the terminal, both support `--json`, and neither writes to your database.

---

## `backlog activity analyze` — workflow health at a glance

```sh
backlog activity analyze --project app --since 7d
```

A single report on how a project is trending over a time window:

- **Throughput** — tasks created vs. completed, plus current todo/doing/done counts.
- **Cycle time by type** — how long bugs, features, and chores take from start to close (avg / min / max).
- **Status-transition latency** — how long work sits in `todo` before someone picks it up, and in `doing` before it ships.
- **WIP by actor** — who (human or AI) is holding the most in-flight work.
- **Quality signals** — reopened tasks, bug follow-ups, label churn, and tasks closed with no completion evidence.
- **Human-vs-AI close ratio** — how much of the queue your agents are actually clearing.

`--since` accepts `7d`, `24h`, `all`, an RFC3339 timestamp, or `YYYY-MM-DD`. Add `--json` to feed it into a dashboard or a standup script.

## `backlog doctor project` — a linter for your queue

```sh
backlog doctor project --project app
```

`doctor check` already verified the database. `doctor project` verifies the *work*. It detects:

- Tasks created but **never started** (still `todo`, no status transition ever).
- `doing` tasks that have **gone quiet** past `--stale-after` (default `7d`).
- Tasks **missing a plan**.
- Tasks **closed with no completion comment or evidence**.
- Tasks whose only recent activity is a **label change**.
- **Final-audit tasks marked done** while earlier work in the project is still open.

Every issue carries a `severity`, a machine-readable `code`, and the `evidence` behind it — so the next agent can fix it from the report alone, instead of rediscovering the problem.

---

## Also in this release

**Improved**
- `backlog install-skills` now installs Codex skills into `~/.codex/skills/<name>/SKILL.md` with Codex-compatible frontmatter, instead of writing saved prompts.
- The Docs web UI can download all visible docs from the list view in one click.

**Fixed**
- Hide the all-docs download action while a single document is open in the reader.

---

## Install / upgrade

```sh
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -L https://github.com/mazen160/backlog/releases/latest/download/backlog_${OS}_${ARCH}.tar.gz | tar xz
sudo mv backlog /usr/local/bin/
```

Or from source:

```sh
go install github.com/mazen160/backlog/cmd/backlog@latest
```

One binary, no dependencies, no telemetry. Full history in [CHANGELOG.md](CHANGELOG.md).

**Full Changelog**: https://github.com/mazen160/backlog/compare/v1.0.2...v1.0.3
