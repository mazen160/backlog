# Backlog v1.0.4: Public Release

This release gets Backlog ready for its broader public launch. The CLI and local-first database model are unchanged; the work here is the website, docs, embedded skills, and a few web UI edges that showed up while tightening the public docs.

## Public release polish

- Refreshed the README, website, docs, API reference, MCP pages, skill docs, and wiki copy.
- Added updated brand, banner, Open Graph, and website imagery so shared links and docs look consistent.

## Skill cleanup

The project-memory skills are now one `backlog-memory` skill instead of separate learn/store variants.

Use:

```text
/backlog-memory learn
/backlog-memory store
```

or call `/backlog-memory` and let it choose the right mode from context. Embedded skill files now use `SKILL.md` casing consistently for Codex-compatible installs.

## Web UI fixes

- Long task titles wrap correctly in board cards, task headers, and header toolbars.
- The web UI now has a sidebar collapse toggle for tighter workspaces.

## Install or upgrade

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

**Full Changelog**: https://github.com/mazen160/backlog/compare/v1.0.3...v1.0.4
