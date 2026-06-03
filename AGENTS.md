# arc — agent notes

Go + Cobra CLI for Linux/macOS maintenance. Layout: `main.go` → `cmd/` (Cobra) → `internal/*`.

## Build / test

```bash
make build
go build -o bin/arc .
ARC_EXTRA_COMMANDS= go test ./...
GOOS=darwin go test -exec=true ./...   # macOS compile-oriented check from Linux
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
- `internal/platform` — typed GOOS detection (`Detect`, `Parse`, `OS.String`)
- `internal/pkgmgr` — package-management boundary selected once in `cmd.App`
- `internal/syscontrol` — system-control boundary selected once in `cmd.App`
- `internal/hardware` — platform-specific hardware reporting boundary
- `internal/setupdeps` — setup/dependency installer boundary
- `internal/deps` — platform-specific validate tool lists
- `internal/brew` — Homebrew helpers for macOS package commands
- `internal/pacman` — pacman helpers for Linux package commands
- `internal/sysupdate` — Linux `arc update system` workflow
- `internal/gitcleanup` — `arc git cleanup` logic
- `internal/selfupdate` — `arc update self`
- `internal/ai` — `arc ai usage` (live provider quota, networked) and `arc ai tokens` (local Claude/Codex session-log token history priced via built-in `internal/ai/pricing.go`, no network). See `docs/ai_tokens.md`.

## Adding commands

New command: new file under `cmd/`, register in `init()`, add the path string to `expectedCommands` in `cmd/commands_test.go`.

Output: `internal/output` (`Info`, `Warning`, `Table`, …). JSON: global `-j` / `--json` on the root command.

## Updates

- `arc update system` — Linux: pacman / yay / cache (`--no-aur`, `--no-cache`); macOS: Homebrew update / upgrade / cleanup (`--no-cache`)
- `arc update self` — upgrade the `arc` binary (`internal/selfupdate`)
- `arc update uv` — `uv self update`

## Platform behavior

Keep platform behavior explicit. Detect the platform once in `cmd.App`, then inject concern-specific implementations (`pkgmgr`, `syscontrol`, `hardware`, `setupdeps`). Do not put new `runtime.GOOS` or ad hoc platform checks inside command handlers; the allowed exception is validating platform-specific user intent such as `--aur-only` or `--no-aur`.

Linux-only flags (`--aur-only`, `--no-aur`) should return a clear unsupported-platform error with non-zero exit on macOS when explicitly used. `arc parts` is human-readable and platform-specific; do not promise identical output across platforms.

## `internal/skills`

Implements `arc skills` and `arc rules`: canonical `~/ai/skills/`, symlinks into provider dirs, `SKILL.md` + YAML frontmatter. Tests use `ARC_*` path overrides (see `paths.go`).

## Release

Version via `-ldflags` (`Makefile`). License: MIT.
