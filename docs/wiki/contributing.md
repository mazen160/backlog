# Contributing

## Prerequisites

- **Go 1.25** or later (`go version`)
- **make** (standard on macOS/Linux; available via Chocolatey or WSL on Windows)
- **Node.js + npx** — only needed for E2E tests (`make test-e2e`)

## Build

```sh
git clone https://github.com/mazen160/backlog
cd backlog
make build
```

Output binary: `./backlog` in the repo root.

`make build` runs `go build -o backlog ./cmd/backlog`. The version string is injected via ldflags from the Makefile.

## Test

```sh
make test        # unit tests + service-layer integration tests
make test-e2e    # Playwright browser tests (requires binary to be built first)
make cover       # test + HTML coverage report
make fmt         # gofmt
make vet         # go vet
```

The service tests create a temporary SQLite database in-memory and exercise the full service → repo → SQLite stack. They do not require an existing workspace.

E2E tests live in `e2e/`. The setup flow:

1. `globalSetup.js` — kills any leftover server, wipes `e2e/.test.db`, seeds data via the binary, spawns the web server on port 8181 with `BACKLOG_DB=e2e/.test.db`.
2. Tests run against `http://localhost:8181`.
3. `globalTeardown.js` — kills the server via the stored PID file.

Run E2E tests separately:

```sh
make build              # always rebuild binary first
cd e2e && npx playwright test
```

## Adding a new CLI command

1. Create `internal/cli/<name>.go` with a `new<Name>Cmd() *cobra.Command` function.
2. Register it in `internal/cli/root.go` inside `newRootCmd()`:
   ```go
   root.AddCommand(
       // …existing commands…
       newMyNewCmd(),
   )
   ```
3. If the command needs DB access, it should use `app.DB`, `app.Actor`, and `app.Out` — these are populated by `openApp()` in `PersistentPreRunE` before any subcommand runs.
4. If the command should work without a DB (like `version` or `profile`), add its name to the skip list in `PersistentPreRunE`.
5. Add tests in `internal/cli/<name>_test.go` or extend the service tests in `internal/service/`.

## Adding a DB migration

1. Create `internal/migrate/migrations/NNNN_description.sql` where `NNNN` is the next sequential number (e.g., `0009_new_table.sql`).
2. Write the SQL. Use `IF NOT EXISTS` and `IF EXISTS` guards where appropriate so the migration is safe to inspect in isolation.
3. The migration runner in `internal/migrate/migrate.go` will pick it up automatically on the next DB open. Migrations are applied in lexicographic filename order, each in its own transaction.
4. Never modify an existing migration file — they are immutable once committed. To change a schema, write a new migration.

## Profile gotcha in manual testing

The service tests register a profile named `e2etest` and set it as the default. If a test exits uncleanly, this can leave `e2etest` as the default profile in `~/.config/backlog/config.toml`. Subsequent `./backlog` commands without a `--profile` flag will target the test database, not your real workspace.

**Always pass `--profile default`** when running the CLI from inside the repo:

```sh
./backlog --profile default task list
./backlog --profile default task add -p myproject -t "..." --as "ai:claude-sonnet-4-6"
```

This ensures you are always writing to the intended workspace regardless of what the tests did to the default profile.

## Binary rebuild requirement

After editing any Go source file, the `./backlog` binary in the repo root must be rebuilt before testing CLI changes:

```sh
go build -o backlog ./cmd/backlog
```

`go build ./...` verifies compilation but does **not** update the binary. Running `make build` is the canonical way to rebuild.

## Code structure reference

```
cmd/backlog/main.go        Entry point; injects version via ldflags
internal/cli/              Cobra command handlers
internal/service/          Business logic services (TaskService, PlanService, …)
internal/repo/             Raw SQL repository layer (no ORM)
internal/models/           Shared domain types
internal/config/           TOML config loading and merging
internal/profile/          Profile registry (read/write ~/.config/backlog/config.toml)
internal/migrate/          Schema migration runner + embedded SQL files
internal/web/              HTTP server, API handlers, embedded SPA
internal/mcpserver/        MCP stdio JSON-RPC 2.0 server
internal/output/           CLI output formatting (table, JSON)
e2e/                       Playwright end-to-end tests
skills/                    Skill markdown files (embedded via skills/skills.go)
```

Architecture diagram: [architecture.md](architecture.md).
Data model details: [data-model.md](data-model.md).
