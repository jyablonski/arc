# arc

A single, personal CLI for system maintenance and AI-tooling workflows on Arch Linux and macOS.

## Overview

`arc` is a small, self-contained Go binary that pulls the everyday commands I run across my machines (system updates, package cleanup, hardware inspection, and keeping tabs on my AI coding tools) behind one consistent interface. Instead of memorizing many platform-specific commands or maintaining a ton of shell aliases, I run `arc <thing>` and get the same command, flags, and output whether I'm on Arch or a Mac.

Under the hood it wraps the native tools each platform already ships (pacman/yay on Arch, Homebrew on macOS) and reads the local state that tools like Claude Code and Codex write to disk. `arc` adds a few things: safer defaults, clearer failure messages, consistent `--json` output, and platform detection so one command does the right thing everywhere.

## What it offers

- System maintenance: one-shot system updates, cache and orphan cleanup, package statistics, hardware and system inspection, and dependency validation, each dispatching to the right package manager for the host.
- AI coding-tool observability: live subscription/quota usage across Claude, Codex, and Cursor (`ai usage`), plus historical local token consumption with an API-equivalent cost estimate mined from on-disk session logs (`ai tokens`).
- Shared AI configuration: a canonical store for AI skills, `AGENTS.md` rules, and MCP servers, kept in sync across Claude, Codex, Cursor, and opencode (`skills`, `rules`, `mcp`).
- Self-management: self-update from GitHub releases, one-command dependency setup, and local-only usage stats for `arc` itself.

Instead of remembering scattered commands or maintaining shell aliases:

```bash
# Before
sudo pacman -Syu && yay -Syu --aur --diffmenu --editmenu && sudo paccache -rv
pacman -Qi | awk '/^Name/ {name=$3} /^Installed Size/ {print $4, $5, name}' | sort -h | tail -25
# Check Claude, Codex, and Cursor usage separately

# After
arc update system
arc packages --top 25
arc ai usage
arc ai tokens
```

## How it works

- One command, two platforms: `arc` detects the host and dispatches to the native tooling (pacman/yay/paccache on Arch, Homebrew on macOS), so the same command works on both. Arch-only flags such as `--aur-only` return a clear unsupported-platform error on macOS rather than failing silently.
- AUR takeover triage: before yay runs, `arc update system` checks pending AUR updates for takeover signals (maintainer changes, orphan adoptions, deletions, one account grabbing many packages) and scans changed package files for high-signal patterns, flagging only what's new since the last trusted run. It surfaces findings; you still decide at yay's diffmenu. See [platforms](docs/platforms.md) for details.
- Reads local state, never re-authenticates: the AI commands never run their own login flows. `ai usage` reads whatever credentials each vendor tool already stored (Claude's OAuth token, Codex's app-server session, Cursor's local session DB) and calls each provider's usage API, isolating failures per provider. `ai tokens` is fully offline: it scans the session transcripts Claude Code and Codex write to `~/.claude` and `~/.codex` and prices them from a local pricing table.
- Consistent output: every command supports `--json` for scripting, with colored, human-readable output by default.
- Local and private by default: usage tracking and token accounting stay on the machine; nothing is uploaded.

## Installation

Download the latest release. Most people want amd64, on Linux or an Intel Mac (swap `arc-linux-amd64` for `arc-darwin-amd64` on an Intel Mac):

```bash
curl -L https://github.com/jyablonski/arc/releases/latest/download/arc-linux-amd64 -o ~/.local/bin/arc
chmod +x ~/.local/bin/arc
```

On an Apple Silicon Mac:

```bash
curl -L https://github.com/jyablonski/arc/releases/latest/download/arc-darwin-arm64 -o ~/.local/bin/arc
chmod +x ~/.local/bin/arc
```

Unsigned GitHub release binaries may be quarantined by macOS. If macOS blocks the first run, approve it in System Settings or remove the quarantine attribute:

```bash
xattr -d com.apple.quarantine ~/.local/bin/arc
```

Make sure `~/.local/bin` is in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Or build from source:

```bash
git clone https://github.com/jyablonski/arc.git
cd arc

# this builds the binary and installs it to ~/.local/bin
make install
```

Starting from version 0.4.0, if you already have `arc` installed, you can update the binary with:

```bash
arc update self
```

## Commands

### System maintenance

| Command         | Arch Linux                     | macOS                         | Example                    |
| --------------- | ------------------------------ | ----------------------------- | -------------------------- |
| `update system` | pacman, yay, paccache          | Homebrew                      | `arc update system`        |
| `update uv`     | `uv self update`               | `uv self update`              | `arc update uv`            |
| `clean`         | pacman cache and orphans       | `brew cleanup` / `autoremove` | `arc clean --orphans-only` |
| `packages`      | pacman stats and size info     | Homebrew package summary      | `arc packages --json`      |
| `installed`     | pacman explicit packages / AUR | Homebrew formulae             | `arc installed --count`    |
| `info`          | fastfetch                      | fastfetch                     | `arc info`                 |
| `parts`         | dmidecode, lspci, lshw         | system_profiler, sysctl       | `arc parts`                |
| `sleep`         | systemd suspend                | pmset sleep                   | `arc sleep`                |

### AI tooling

| Command       | Description                                                | Example           |
| ------------- | ---------------------------------------------------------- | ----------------- |
| `ai usage`    | Live subscription/quota usage across Claude, Codex, Cursor | `arc ai usage`    |
| `ai tokens`   | Historical local token usage with API-equivalent cost      | `arc ai tokens`   |
| `ai sessions` | List recent local Claude and Codex sessions                | `arc ai sessions` |
| `ai health`   | Offline auth/tooling/config health check across AI tools   | `arc ai health`   |
| `ai pricing`  | Refresh the local model pricing cache used by `ai tokens`  | `arc ai pricing`  |
| `skills`      | Sync shared AI/LLM skills across providers                 | `arc skills sync` |
| `rules`       | Sync the shared `AGENTS.md` rules file across providers    | `arc rules sync`  |
| `mcp`         | Sync shared MCP servers across providers                   | `arc mcp sync`    |

### arc itself

| Command       | Description                             | Example            |
| ------------- | --------------------------------------- | ------------------ |
| `update self` | Update `arc` to the latest release      | `arc update self`  |
| `setup`       | Install the tools `arc` depends on      | `arc setup`        |
| `validate`    | Check that required tools are in `PATH` | `arc validate`     |
| `stats`       | Show local `arc` command usage stats    | `arc stats --json` |

Arch-only flags such as `--aur-only` and `--no-aur` return an unsupported-platform error on macOS when explicitly used. Every command supports `--json` (`-j`); use `arc <command> --help` for detailed flag information.

## Additional Documentation

More detailed notes are available in `docs/`:

- [AI usage](docs/ai_usage.md)
- [AI tokens](docs/ai_tokens.md)
- [AI pricing](docs/ai_pricing.md)
- [Shared skills](docs/skills.md)
- [Shared rules](docs/rules.md)
- [Shared MCP servers](docs/mcp.md)

## Development

```bash
make build     # Build binary
make install   # Build and install to ~/.local/bin
make test      # Run tests
make fmt       # Format code
make lint      # Run linter
```

## License

MIT
