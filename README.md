# arc

A personal CLI tool for system management and maintenance on Arch Linux.

## What It Does

`arc` consolidates common system tasks into a single command-line tool.
It provides a consistent interface for system operations with better argument handling, help text, colored output, and error handling.

```bash
arc update               # Run system updates (pacman, yay, cache cleanup)
arc aws rotate-keys      # Rotate AWS IAM access keys
arc docker clean         # Clean Docker resources (images, containers, volumes)
```

## Why

Instead of remembering scattered commands or maintaining shell aliases:

```bash
# Before
sudo pacman -Syu && yay -Syu --aur && sudo paccache -rv
pacman -Qi | awk '/^Name/ {name=$3} /^Installed Size/ {print $4, $5, name}' | sort -h | tail -25

# After
arc update
arc packages --top 25
```

## Installation

**Download the latest release:**

```bash
curl -L https://github.com/jyablonski/arc/releases/latest/download/arc-linux-amd64 -o ~/.local/bin/arc
chmod +x ~/.local/bin/arc
```

Make sure `~/.local/bin` is in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

**Or build from source:**

```bash
git clone https://github.com/jyablonski/arc.git
cd arc

# this builds the binary and installs it to ~/.local/bin
make install
```

Starting from version 0.3.0, if you already have `arc` installed, you can update it using:

```bash
arc self update
```

## Commands

| Command       | Description                        | Example                        |
| ------------- | ---------------------------------- | ------------------------------ |
| `update`      | System updates via pacman and yay  | `arc update --no-aur`          |
| `clean`       | Remove cached packages and orphans | `arc clean --orphans-only`     |
| `packages`    | Package stats and size info        | `arc packages --top 10 --json` |
| `info`        | System information                 | `arc info`                     |
| `parts`       | Hardware components                | `arc parts`                    |
| `installed`   | List installed packages            | `arc installed --aur-only`     |
| `search`      | Search package repos               | `arc search neovim`            |
| `sleep`       | Suspend system                     | `arc sleep`                    |
| `validate`    | Check dependencies                 | `arc validate`                 |
| `setup`       | Install required tools             | `arc setup`                    |
| `self update` | Update arc to the latest version   | `arc self update`              |
| `skills`      | Manage shared AI/LLM skills        | `arc skills sync`              |
| `rules`       | Manage shared AGENTS.md rules      | `arc rules sync`               |

Use `arc <command> --help` for detailed flag information.

### Shared AI skills

`arc skills` manages a canonical `~/ai/skills/<name>/SKILL.md` store and symlinks each skill into every AI provider directory (Claude, Codex, Cursor, opencode).
Validation of frontmatter is strict.
Provider slots that already hold real content are never clobbered.

```bash
# Promote a draft directory into canonical and link everywhere.
arc skills add ./my-draft
# Scaffold a new skill from a template.
arc skills add --new my-skill
# Migrate provider-local skills and forward-link (idempotent).
arc skills sync
# Show canonical skills and per-provider symlink status.
arc skills list
# Check frontmatter; auto-rename directory on name mismatch.
arc skills validate --fix
# Remove canonical copy and sweep provider symlinks (real content is left alone).
arc skills remove my-skill
# Remove only dangling symlinks.
arc skills prune
```

### Shared rules file

`arc rules` symlinks `~/ai/AGENTS.md` into each provider's rules-file slot (`~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, `~/.config/opencode/AGENTS.md`).

```bash
# Ensure symlinks are in place (creates parent directories as needed).
arc rules sync

# Show per-provider symlink status.
arc rules status
```

## Development

```bash
make build     # Build binary
make install   # Build and install to ~/.local/bin
make test      # Run tests
make fmt       # Format code
make lint      # Run linter
```

## Project Structure

```
├── main.go
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go           # root command, global flags
│   ├── update.go         # system updates (pacman, yay, cache cleanup)
│   ├── clean.go          # package cache and orphan removal
│   ├── ...               # each arc command gets its own file here
├── internal/
│   ├── shell/
│   │   └── exec.go       # wrapper for running shell commands
│   ├── output/
│   │   └── format.go     # manages colored output, tables, formatting
│   ├── pacman/
│   │   └── pacman.go     # pacman-specific parsing and helpers
│   └── skills/           # shared AI skills, provider paths, sync, validation
```

## License

MIT
