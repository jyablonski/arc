# arc — agent notes

Go + Cobra CLI for Arch maintenance. Layout: `main.go` → `cmd/` (Cobra) → `internal/*`.

## Build / test

```bash
make build
go build -o bin/arc .
ARC_EXTRA_COMMANDS= go test ./...
```

`ARC_EXTRA_COMMANDS=1` in the environment breaks `cmd` tests that diff the full command tree unless you are testing those commands.

## Adding commands

New command: new file under `cmd/`, register in `init()`, add the path string to `expectedCommands` in `cmd/commands_test.go`.

Output: `internal/output` (`Info`, `Warning`, `Table`, …). JSON: global `-j` / `--json` on the root command.

## `internal/skills`

Implements `arc skills` and `arc rules`: canonical `~/ai/skills/`, symlinks into provider dirs, `SKILL.md` + YAML frontmatter. Tests use `ARC_*` path overrides (see `paths.go`).

## Release

Version via `-ldflags` (`Makefile`). License: MIT.
