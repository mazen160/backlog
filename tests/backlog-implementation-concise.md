---
marp: true
theme: default
class: invert
paginate: true
style: |
  :root {
    --color-background: #0d1117;
    --color-foreground: #e6edf3;
    --color-highlight: #e3b341;
    --color-accent: #58a6ff;
    --color-muted: #8b949e;
    --color-surface: #161b22;
    --color-border: #30363d;
    font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  }

  section {
    background: #0d1117;
    color: #e6edf3;
    font-family: -apple-system, 'Segoe UI', system-ui, sans-serif;
    padding: 48px 64px;
  }

  section.title {
    background: #0d1117;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  h1 {
    color: #e6edf3;
    font-size: 2.4em;
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: -0.02em;
    margin-bottom: 0.2em;
  }

  h2 {
    color: #e3b341;
    font-size: 1.05em;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    margin-bottom: 0.6em;
    margin-top: 0;
  }

  h3 {
    color: #58a6ff;
    font-size: 1em;
    font-weight: 600;
    margin-bottom: 0.3em;
  }

  p {
    color: #c9d1d9;
    line-height: 1.6;
    font-size: 0.95em;
  }

  li {
    color: #c9d1d9;
    line-height: 1.65;
    font-size: 0.92em;
  }

  strong {
    color: #e6edf3;
    font-weight: 700;
  }

  em {
    color: #e3b341;
    font-style: normal;
    font-weight: 600;
  }

  code {
    background: #161b22;
    color: #e3b341;
    border: 1px solid #30363d;
    padding: 0.1em 0.4em;
    border-radius: 4px;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 0.88em;
  }

  pre {
    background: #161b22 !important;
    border: 1px solid #30363d;
    border-radius: 8px;
    padding: 18px 22px;
    margin: 14px 0;
  }

  pre code {
    background: transparent;
    border: none;
    padding: 0;
    color: #c9d1d9;
    font-size: 0.82em;
    line-height: 1.5;
  }

  blockquote {
    border-left: 3px solid #e3b341;
    background: #161b22;
    padding: 14px 20px;
    margin: 16px 0;
    border-radius: 0 6px 6px 0;
    font-size: 0.95em;
    color: #c9d1d9;
  }

  blockquote p {
    margin: 0;
  }

  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.86em;
  }

  th {
    background: #161b22;
    color: #e3b341;
    padding: 10px 14px;
    text-align: left;
    border-bottom: 2px solid #30363d;
    font-weight: 600;
    letter-spacing: 0.04em;
    font-size: 0.84em;
    text-transform: uppercase;
  }

  td {
    padding: 10px 14px;
    border-bottom: 1px solid #21262d;
    color: #c9d1d9;
  }

  tr:last-child td {
    border-bottom: none;
  }

  footer,
  section::after {
    color: #484f58;
    font-size: 0.72em;
  }
---

<!-- _class: title -->
<!-- _paginate: false -->

## Demo

# Backlog
### Local-first task queue for AI coding agents

**Mazin Ahmed** · github.com/mazen160/backlog

---

## The problem

AI agents forget work when the session ends.

- plans disappear
- decisions disappear
- comments disappear
- cross-IDE handoff breaks

> The chat has context. The project does not.

---

## Why current workflows fail

| Workflow | Cost |
|---|---|
| Long chat thread | context bloat |
| Switch IDEs | no shared state |
| Resume later | no durable history |
| Markdown notes | manual coordination |

**Fix:** move work state out of chat and into a shared local system.

---

## What Backlog is

A **single Go binary** with:

- SQLite task database
- built-in MCP server
- CLI + web UI
- shared state across agents

```bash
backlog init
backlog install-skills
```

---

## Architecture

```text
backlog.db (SQLite)
  tasks · plans · comments · memory · activity
          │
      MCP server
          │
 Claude Code · Cursor · Codex · OpenCode
```

**One database. Every agent reads and writes the same state.**

---

## What persists

| Layer | Purpose |
|---|---|
| Tasks | status, priority, owner |
| Plans | versioned task plans |
| Comments | append-only discussion |
| Memory | reusable project knowledge |
| Activity | audit trail |

This is what makes work resumable.

---

## Memory is the leverage point

```bash
backlog memory add \
  --project api \
  --body "JWT TTL = 15min" \
  --tag "decision,auth" \
  --as ai:claude-code
```

```bash
backlog memory list --project api --tag auth
```

Memory keeps decisions available to the next session.

---

## The agent loop

```bash
backlog task list --status doing --json
backlog memory list --project api --tag decision --json
backlog activity --limit 20 --json
```

**Pick → Plan → Ship → Review → Repeat**

No need for “where were we?”

---

## Built for agent workflows

- **Local-first**: no SaaS, low latency
- **MCP-native**: one protocol, many clients
- **Single binary**: simple install
- **Actor attribution**: who changed what

```text
human:alice      created TASK-42
ai:claude-code   updated TASK-42
```

---

## What I learned

### Better tasks beat better prompting

- vague task: expensive, slow, error-prone
- scoped task: faster, cheaper, more reliable

### Fresh subagents beat long threads

| Mode | Tokens |
|---|---|
| Long thread | ~500k |
| Fresh subagents | 30–60k |

---

## Try it

```bash
go install github.com/mazen160/backlog/cmd/backlog@latest

backlog init
backlog install-skills
```

| | |
|---|---|
| Repo | github.com/mazen160/backlog |
| Docs | mazen160.github.io/backlog |
| MCP setup | mazen160.github.io/backlog/mcp.html |
| License | MIT |

**Goal:** make agent work resumable, shared, and cheap.
