# arc

A personal CLI tool for system management and maintenance on Arch Linux.

## What It Does

`arc` consolidates common system tasks into a single command-line tool. It provides a consistent interface for system operations with better argument handling, help text, colored output, and error handling.

## Why

`arc` is for maintenance tasks that are too important or too fiddly to leave as one-off shell snippets.

It wraps those workflows with:

- Safer defaults and clearer failure messages
- Consistent flags, JSON output, and help text
- Reusable checks before running system-changing commands
- One place to evolve personal Arch, Docker, AWS, and AI-tooling workflows

Instead of remembering scattered commands or maintaining shell aliases:

```bash
# Before
sudo pacman -Syu && yay -Syu --aur && sudo paccache -rv
pacman -Qi | awk '/^Name/ {name=$3} /^Installed Size/ {print $4, $5, name}' | sort -h | tail -25
aws sts get-caller-identity && aws iam create-access-key --user-name "$USER" && aws configure
# Check Claude, Codex, and Cursor usage separately

# After
arc update system
arc packages --top 25
arc aws rotate-keys
arc ai usage
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

Starting from version 0.4.0, if you already have `arc` installed, you can update the binary with:

```bash
arc update self
```

## Commands

| Command         | Description                        | Example                        |
| --------------- | ---------------------------------- | ------------------------------ |
| `update system` | System updates via pacman and yay  | `arc update system`            |
| `update self`   | Update `arc` to the latest release | `arc update self`              |
| `update uv`     | Update the `uv` tool               | `arc update uv`                |
| `clean`         | Remove cached packages and orphans | `arc clean --orphans-only`     |
| `packages`      | Package stats and size info        | `arc packages --top 10 --json` |
| `info`          | System information                 | `arc info`                     |
| `parts`         | Hardware components                | `arc parts`                    |
| `installed`     | List installed packages            | `arc installed --aur-only`     |
| `search`        | Search package repos               | `arc search neovim`            |
| `sleep`         | Suspend system                     | `arc sleep`                    |
| `validate`      | Check dependencies                 | `arc validate`                 |
| `setup`         | Install required tools             | `arc setup`                    |
| `ai usage`      | Show AI coding tool usage          | `arc ai usage`                 |
| `skills`        | Manage shared AI/LLM skills        | `arc skills sync`              |
| `rules`         | Manage shared AGENTS.md rules      | `arc rules sync`               |

Use `arc <command> --help` for detailed flag information.

## Additional Documentation

More detailed notes are available in `docs/`:

- [AI usage](docs/ai_usage.md)
- [Shared skills](docs/skills.md)
- [Shared rules](docs/rules.md)

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
│   ├── update.go         # arc update {system,self,uv}
│   ├── clean.go          # package cache and orphan removal
│   ├── ...               # one file per command group (Cobra wiring only)
├── internal/
│   ├── arcerrs/          # shared sentinel errors for CLI exit behavior
│   ├── extracmd/         # ARC_EXTRA_COMMANDS visibility for admin-only commands
│   ├── shell/
│   │   └── exec.go       # wrapper for running shell commands
│   ├── output/
│   │   └── format.go     # manages colored output, tables, formatting
│   ├── pacman/
│   │   └── pacman.go     # pacman-specific parsing and helpers (+ kernel list)
│   ├── selfupdate/       # arc update self
│   ├── sysupdate/        # arc update system
│   ├── ghworkflow/       # arc gh restart-dashboard
│   ├── gitcleanup/       # arc git cleanup
│   └── skills/           # shared AI skills, provider paths, sync, validation
```

## License

MIT
