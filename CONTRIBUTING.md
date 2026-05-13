# Contributing to Backlog

Thank you for your interest in contributing. This document covers everything you need to get started.

## Prerequisites

- Go 1.22 or later
- `make` (GNU Make)
- `git`

Optional:
- `golangci-lint` for extended linting (`make lint`)
- `goreleaser` for local release testing (`make snapshot`)

## Getting started

```sh
git clone https://github.com/mazen160/backlog
cd backlog
make build   # produces ./backlog
make test    # runs all unit + e2e tests
```

## Project structure

```
cmd/backlog/          main entrypoint (version injection)
internal/
  cli/                Cobra command definitions
  config/             workspace config and global profile registry
  ids/                ULID generation
  mcpserver/          MCP stdio server (JSON-RPC 2.0)
  migrate/            schema migration runner + SQL files
  migrate/migrations/ *.sql migration files (numbered)
  models/             shared types (Task, Plan, Actor, …)
  output/             table / JSON printer
  profile/            named workspace registry
  repo/               SQLite repository layer
  service/            business logic (task, plan, comment, label, …)
  timeutil/           Unix-nanosecond helpers
tests/                end-to-end tests (builds binary, runs subprocesses)
skills/backlog/       Claude Code skill for AI agent use
docs/                 user documentation
```

## Running tests

```sh
make test           # all tests, 120s timeout
make test-verbose   # with -v flag
make cover          # test + open HTML coverage report
```

The e2e suite in `tests/` builds the actual binary and invokes it as a subprocess — run it after any CLI or migration change.

## Code style

- `gofmt` is required: run `make fmt` before committing.
- `go vet` must pass: run `make vet`.
- No `//nolint` without a comment explaining why.
- Default to **no comments** in code. Only add one when the _why_ is non-obvious.
- Error strings are lower-case and do not end with punctuation.

## Adding a database migration

1. Create a new file in `internal/migrate/migrations/` named `NNNN_description.sql` where `NNNN` is the next sequential number.
2. Use `IF NOT EXISTS`, `INSERT OR IGNORE`, and similar idempotent constructs where possible.
3. Never edit an existing migration file — it has already run on all existing databases.
4. Test the migration against an existing database (not just a fresh `backlog init`):

```sh
cp /some/existing/backlog.db /tmp/test.db
./backlog --db /tmp/test.db task list   # triggers migration runner
```

5. Add a test in `internal/service/service_test.go` if the migration changes behavior.

## Commit messages

Use the imperative mood, 72-char subject line:

```
Add --due-date flag to task add and task update
Fix plan/comment commands not resolving TASK-N refs
```

No ticket numbers in the subject. Reference issues in the body if relevant.

## Pull request checklist

Before opening a PR, confirm:

- [ ] `make test` passes
- [ ] `make fmt` and `make vet` produce no output
- [ ] New behavior has a test (unit in `service_test.go` and/or e2e in `tests/e2e_test.go`)
- [ ] If the DB schema changed, a new migration file is included
- [ ] `CHANGELOG.md` has an entry under `## Unreleased`
- [ ] Docs updated if any command, flag, or behavior changed

## Reporting bugs

Open an issue and include:

- `backlog version` output
- OS and architecture
- The exact command you ran
- What you expected vs what happened

## Feature requests

Open an issue describing the use case before implementing. For small improvements, a PR with a description is fine directly.
