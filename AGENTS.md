# arc — agent notes

Go + Cobra CLI for Linux/macOS maintenance. Layout: `main.go` → `cmd/` (Cobra) → `internal/*`.

## Build / test

```bash
make build
go build -o bin/arc .
go test ./...
GOOS=darwin go test -exec=true ./...   # macOS compile-oriented check from Linux
make mocks   # optional — regenerates moq `*_moq.go` after interface edits
make coverage       # filtered coverage.out (+ coverage.full.out raw)
make test-ci        # CI: tests + JUnit + filtered coverage.out (same filter as coverage)
```

**Coverage reporting:** **`make coverage`**, **`make test-ci`**, and **`make coverage-app`** all write **`coverage.out`** as the **filtered** profile (moq-generated `*_moq.go` lines removed via `awk`), copied from **`coverage.app.out`**. The unfiltered merge stays in **`coverage.full.out`** for debugging. Hand-written code percentages come from `coverage.out`; generated mocks still inflate **`coverage.full.out`** if you inspect it. **`main_test.go`** exercises **`cmd.Execute`** in-process and runs a **`go build` + `arc help`** subprocess smoke test (`TestArcBinary_smoke` skips when `-short`).

**Mocks:** External-ish interfaces use [moq](https://github.com/matryer/moq) (`go.mod` `tool` directive). Shared stubs live in `internal/boundary` (`HTTPDoer`, `ShellRunner`); `internal/ai.Provider` and `notify.Notifier` generate mocks in-package. Regenerate with `make mocks` whenever those interfaces change.

## Library layout (non-Cobra)

- `internal/boundary` — narrow interfaces + moq stubs for HTTP / shell boundaries (`DefaultShell` delegates to `internal/shell`)
- `internal/arcerrs` — sentinel errors returned from commands
- `internal/platform` — typed GOOS detection (`Detect`, `Parse`, `OS.String`)
- `internal/pkgmgr` — package-management boundary selected once in `cmd.App`
- `internal/syscontrol` — system-control boundary selected once in `cmd.App`
- `internal/hardware` — platform-specific hardware reporting boundary
- `internal/setupdeps` — setup/dependency installer boundary
- `internal/deps` — platform-specific validate tool lists
- `internal/brew` — Homebrew helpers for macOS package commands
- `internal/pacman` — pacman helpers for Linux package commands
- `internal/sysupdate` — Linux `arc update system` workflow
- `internal/aurreview` — pre-yay AUR takeover triage (provenance baseline, snapshot diff scan, cluster detection); state under `~/.local/state/arc/`
- `internal/mcp` — `arc mcp` shared MCP server config (see below)
- `internal/gitcleanup` — `arc git cleanup` logic
- `internal/selfupdate` — `arc update self`
- `internal/ai` — `arc ai usage` (live provider quota, networked) and `arc ai tokens` (local Claude/Codex session-log token history, offline). Pricing layers a hand-edited override (`~/.config/arc/ai-pricing.json`) over a fetched cache (`~/.cache/arc/ai-pricing.json`, refreshed by `arc ai pricing`) over built-in defaults (`internal/ai/pricing.go`). See `docs/ai_tokens.md`.

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

Implements `arc skills` and `arc rules`: canonical `~/ai/skills/`, symlinks into provider dirs, each skill a `~/ai/skills/<name>/SKILL.md` with YAML frontmatter. Tests use `ARC_*` path overrides (see `paths.go`).

## `internal/mcp`

Implements `arc mcp`: canonical `~/ai/mcp.json` rendered into each provider's dialect. Deliberately **not** symlink-based — Claude, Codex, and opencode keep MCP servers inside a larger file they also own, so `Provider` implementations merge surgically (order-preserving JSON via `jsonobj.go`, line-splicing TOML via `toml.go`) and rewrite only servers `arc` owns per `~/.config/arc/mcp-state.json`. Adding a provider means implementing `Provider` (`Name`/`ConfigPath`/`Supports`/`Normalize`/`Read`/`Write`/`OmitsDisabled`) and registering it in `DefaultProviders`. All writes go through `writeFileAtomic` (temp file + rename, existing mode preserved) — never `os.WriteFile`: sync rewrites files the AI tools own and keep unrelated state in. `Normalize` must report the round-trip through that dialect or `list` will show permanent drift. Health checks live in `health.go` and are wired into `arc ai health` from `cmd/ai.go`. Tests use `ARC_*` path overrides (see `paths.go`).

## Release

Version via `-ldflags` (`Makefile`). License: MIT.
