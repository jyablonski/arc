# arc — agent notes

Go + Cobra CLI for Arch maintenance. Layout: `main.go` → `cmd/` (Cobra) → `internal/*`.

## Build / test

```bash
make build
go build -o bin/arc .
ARC_EXTRA_COMMANDS= go test ./...
make mocks   # optional — regenerates moq `*_moq.go` after interface edits
make coverage       # filtered coverage.out (+ coverage.full.out raw)
make test-ci        # CI: tests + JUnit + filtered coverage.out (same filter as coverage)
```

`ARC_EXTRA_COMMANDS` is unset for baseline command-tree tests in `commands_test.go` so CI and local shells with `ARC_EXTRA_COMMANDS=1` stay deterministic. Visibility for those commands is `internal/extracmd`.

**Coverage reporting:** **`make coverage`**, **`make test-ci`**, and **`make coverage-app`** all write **`coverage.out`** as the **filtered** profile (moq-generated `*_moq.go` lines removed via `awk`), copied from **`coverage.app.out`**. The unfiltered merge stays in **`coverage.full.out`** for debugging. Hand-written code percentages come from `coverage.out`; generated mocks still inflate **`coverage.full.out`** if you inspect it. **`main_test.go`** exercises **`cmd.Execute`** in-process and runs a **`go build` + `arc help`** subprocess smoke test (`TestArcBinary_smoke` skips when `-short`).

**Mocks:** External-ish interfaces use [moq](https://github.com/matryer/moq) (`go.mod` `tool` directive). Shared stubs live in `internal/boundary` (`HTTPDoer`, `ShellRunner`); `internal/ai.Provider` and `notify.Notifier` generate mocks in-package. Regenerate with `make mocks` whenever those interfaces change.

## Library layout (non-Cobra)

- `internal/boundary` — narrow interfaces + moq stubs for HTTP / shell boundaries (`DefaultShell` delegates to `internal/shell`)
- `internal/arcerrs` — sentinel errors returned from commands
- `internal/extracmd` — `ARC_EXTRA_COMMANDS` gating and admin command visibility
- `internal/sysupdate` — `arc update system` workflow
- `internal/ghworkflow` — `arc gh restart-dashboard` logic
- `internal/gitcleanup` — `arc git cleanup` logic
- `internal/selfupdate` — `arc update self`

## Adding commands

New command: new file under `cmd/`, register in `init()`, add the path string to `expectedCommands` in `cmd/commands_test.go`.

Output: `internal/output` (`Info`, `Warning`, `Table`, …). JSON: global `-j` / `--json` on the root command.

## Updates

- `arc update system` — pacman / yay / cache (`--no-aur`, `--no-cache` on that subcommand)
- `arc update self` — upgrade the `arc` binary (`internal/selfupdate`)
- `arc update uv` — `uv self update`

## `internal/skills`

Implements `arc skills` and `arc rules`: canonical `~/ai/skills/`, symlinks into provider dirs, `SKILL.md` + YAML frontmatter. Tests use `ARC_*` path overrides (see `paths.go`).

## Release

Version via `-ldflags` (`Makefile`). License: MIT.
