# MCP Server

> Skills are the default way to connect an agent (`backlog install-skills`, no server to run); MCP is the alternative for assistants that prefer structured tool calls. See [skill.md](skill.md).

## What is MCP?

The [Model Context Protocol](https://modelcontextprotocol.io) (MCP) is an open standard for connecting AI assistants to external tools and data sources. Instead of embedding tool logic inside a prompt, an MCP server exposes named tools over a standard JSON-RPC 2.0 transport. The AI assistant calls tools by name with structured arguments, and the server returns structured results.

Backlog supports MCP so that any compatible AI coding assistant (Claude Code, Cursor, Codex, OpenCode) can read and write a backlog workspace directly — creating tasks, attaching plans, moving status, leaving comments — without the AI needing to construct CLI invocations.

## Starting the server

```sh
backlog mcp serve --as ai:claude-code --db /path/to/backlog.db
```

The server speaks newline-delimited JSON-RPC 2.0 over stdin/stdout (the standard MCP stdio transport). AI tools spawn the process and communicate over its stdin/stdout. The server stays alive until the parent process closes the pipe.

**`--as`** sets the actor for all write operations. Always set this so writes are attributed correctly. Defaults to `human:$USER` if omitted.

**`--db`** sets the database path explicitly. Alternatively, set `BACKLOG_DB` in the environment.

## `BACKLOG_DB` env var

Using the `BACKLOG_DB` environment variable in the MCP server config is the most reliable way to point the server at a specific workspace. It bypasses the profile resolution chain entirely and works regardless of the working directory from which the AI tool spawns the process.

```
BACKLOG_DB=/path/to/workspace/backlog.db
```

## DB resolution order for MCP

The MCP server uses the same `resolveDB()` chain as the CLI:

1. `--db <path>` flag
2. `$BACKLOG_DB` env var
3. `--profile <name>` flag
4. Default profile from `~/.config/backlog/config.toml`
5. Error if none of the above resolve

See [architecture.md](architecture.md) for details.

## Available MCP tools

| Tool | Required params | Optional params |
|---|---|---|
| `project_list` | — | — |
| `task_create` | `project` (alias), `title` | `description`, `type`, `status`, `priority`, `source`, `external_ref`, `due_date` |
| `task_list` | — | `project`, `status`, `type`, `priority`, `search` |
| `task_show` | `id` | — |
| `task_update` | `id` | `title`, `description`, `status`, `priority`, `due_date` |
| `task_move` | `id`, `status` | — |
| `plan_add` | `task_id`, `title`, `body` | `source` |
| `plan_update` | `plan_id`, `title`, `body` | `change_note` |
| `plan_history` | `plan_id` | — |
| `comment_add` | `task_id`, `body` | — |
| `memory_add` | `project`, `body` | `tags` |
| `memory_list` | `project` | `tag` |
| `doc_add` | `project`, `title`, `body` | — |
| `doc_list` | `project` | — |
| `doc_show` | `id` | — |
| `doc_update` | `id`, `body` | `title`, `change_note` |

**ID formats for tasks**: `task_id` and `id` accept `TASK-N`, bare integer, or full ULID. Plan IDs, doc IDs, and memory IDs are ULIDs only (returned by the corresponding create/list calls).

**Enum values:**
- `type`: `task`, `bug`, `issue`, `improvement`, `feature`, `vulnerability`, `chore`, `spike`, `bucket-list`
- `status`: `todo`, `doing`, `done`
- `priority`: integer `1` (highest/critical) through `5` (lowest/backlog)

## Config examples

### Claude Code (`~/.claude.json`)

```json
{
  "mcpServers": {
    "backlog": {
      "command": "backlog",
      "args": ["mcp", "serve", "--as", "ai:claude-code"],
      "env": {
        "BACKLOG_DB": "/path/to/workspace/backlog.db"
      }
    }
  }
}
```

**Via CLI (recommended):**

```sh
claude mcp add backlog -- backlog mcp serve --as ai:claude-code --db /path/to/workspace/backlog.db
```

**Project-scoped** (`.claude/settings.json` at the repo root):

```json
{
  "mcpServers": {
    "backlog": {
      "command": "backlog",
      "args": ["mcp", "serve", "--as", "ai:claude-code"],
      "env": {
        "BACKLOG_DB": "${workspaceFolder}/backlog.db"
      }
    }
  }
}
```

### Cursor (`~/.cursor/mcp.json` or `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "backlog": {
      "command": "backlog",
      "args": ["mcp", "serve", "--as", "ai:cursor"],
      "env": {
        "BACKLOG_DB": "/path/to/workspace/backlog.db"
      }
    }
  }
}
```

If `backlog` is not on your `PATH`, use the full binary path (e.g., `/usr/local/bin/backlog`).

### Codex (`~/.codex/config.yaml`)

```yaml
mcp_servers:
  backlog:
    command: backlog
    args:
      - mcp
      - serve
      - "--as"
      - "ai:codex"
    env:
      BACKLOG_DB: /path/to/workspace/backlog.db
```

### OpenCode (`~/.config/opencode/config.json` or `opencode.json`)

```json
{
  "mcp": {
    "backlog": {
      "type": "local",
      "command": "backlog",
      "args": ["mcp", "serve", "--as", "ai:opencode"],
      "env": {
        "BACKLOG_DB": "/path/to/workspace/backlog.db"
      }
    }
  }
}
```

## Verifying the connection

Send the MCP handshake manually to confirm the server starts correctly:

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}' \
  | backlog mcp serve --db /path/to/backlog.db
```

Expected response:

```json
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":{"listChanged":false}},"protocolVersion":"2024-11-05","serverInfo":{"name":"backlog","version":"1.0.0"}}}
```
