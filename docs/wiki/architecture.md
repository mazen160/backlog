# Architecture

## High-level overview

```
cmd/backlog/main.go
  └── internal/cli/            Cobra commands (root, task, plan, doc, memory, …)
        └── internal/service/  Business logic (TaskService, PlanService, …)
              └── internal/repo/   SQLite repository (raw SQL, no ORM)
                    └── SQLite (backlog.db)

internal/models/               Shared domain types (Task, Plan, Actor, Comment, …)
internal/config/               WorkspaceConfig + GlobalConfig (TOML)
internal/profile/              Named workspace registry
internal/migrate/              Schema migration runner (sequential SQL files)
internal/web/                  Embedded HTTP server + static SPA (go:embed)
internal/mcpserver/            MCP stdio server (JSON-RPC 2.0)
```

`main.go` injects the version string via ldflags, then calls `cli.Execute()`. From there, each Cobra subcommand in `internal/cli/` constructs a service, calls it, and renders output. Services contain all business logic. Repos contain all raw SQL. Models define types shared across layers.

## DB resolution chain

`resolveDB()` in `internal/cli/root.go` resolves the workspace path in this priority order:

1. `--db <path>` flag — explicit absolute or relative path to `backlog.db`
2. `$BACKLOG_DB` env var — path to `backlog.db`, useful in scripts and MCP configs
3. `--profile <name>` flag — looks up the named profile in `~/.config/backlog/config.toml`, returns `<profile-path>/backlog.db`
4. Default profile — reads `default_profile` from `~/.config/backlog/config.toml` and resolves the same way
5. Error: `"no backlog workspace found — run backlog init to create one"`

There is **no cwd walk-up**. Unlike many tools, backlog does not search parent directories for a `backlog.db` file.

## Actor attribution

Every command that writes to the database requires an actor. The actor is resolved as follows:

1. `--as kind:name` flag (e.g., `--as human:alice` or `--as ai:claude-code`)
2. `defaults.actor` from the workspace `config.toml`
3. `defaults.actor` from the global `~/.config/backlog/config.toml`
4. Fallback: `human:$USER` (or `human:user` if `$USER` is unset)

The actor is stored as two columns in every writable table: `actor_kind` (`human` or `ai`) and `actor_name` (the name string).

## Config layers

Configuration is layered. Priority from highest to lowest:

```
CLI flags  >  workspace config  >  global config  >  hardcoded defaults
```

**Global config** — `~/.config/backlog/config.toml`

Holds the profile registry, user-level output defaults, and user-level task defaults. Example:

```toml
default_profile = "default"
default_project = "backlog"

[profiles]
  [profiles.default]
    path = "~/.backlog/default"
  [profiles.work]
    path = "~/projects/work/.backlog"

[output]
  default_format = "table"
  color = "auto"

[defaults]
  actor = "human:alice"
  priority = 3
  status = "todo"
  type = "task"
```

**Workspace config** — `<workspace-dir>/config.toml`

Same `[output]` and `[defaults]` sections, but scoped to a single workspace. Fields set here override the global config for that workspace. Fields left unset fall through to the global config.

`default_project` can also be set in the workspace config. CLI commands that accept `--project` use it when the flag is omitted; an explicit `--project` flag always wins.

**Effective config** — computed by `config.EffectiveConfig(global, workspace)` in `internal/config/config.go`. CLI flags override the effective config at runtime.

## Profile registry

The profile registry lives in `~/.config/backlog/config.toml` under the `[profiles]` table. Each entry maps a name to a workspace directory path. Paths may use `~` expansion.

Default workspace locations follow the convention `~/.backlog/<profile-name>/`, but any directory can be used via `--path` at `backlog init` time.

The `default_profile` key in the global config determines which profile is used when no `--profile` or `--db` flag is provided.

See [concepts.md](concepts.md) for the definition of Profile and Workspace.

## Embed pattern

Two packages use `//go:embed` to bundle files into the binary:

- `internal/migrate/migrate.go` embeds `migrations/*.sql` — all migration SQL files are compiled into the binary and run automatically on DB open.
- `internal/web/server.go` embeds the `static/` directory — the entire SPA (HTML, CSS, JS) is served from memory, requiring no external files at runtime.
- `skills/skills.go` embeds `backlog/skill.md`, `backlog-enhance-tasks/skill.md`, `backlog-loop/skill.md`, `backlog-goal/skill.md`, and `backlog-memory/skill.md`. `backlog install-skills` writes them into `~/.claude`, `~/.cursor`, `~/.config/opencode`, and `~/.codex` in each tool's native format.

## MCP server

`backlog mcp serve` starts a JSON-RPC 2.0 server on stdio (the standard MCP transport). The AI assistant spawns the process and communicates over stdin/stdout with newline-delimited JSON messages. The server resolves the workspace via the same `resolveDB()` chain as the CLI.

Full MCP setup: [mcp.md](mcp.md).
