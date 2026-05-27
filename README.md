# arc

A personal CLI tool for system management and maintenance on Arch Linux and macOS.

## What It Does

`arc` consolidates common system tasks into a single command-line tool. It provides a consistent interface for system operations with better argument handling, help text, colored output, and error handling.

## Why

`arc` is for maintenance tasks that are too important or too fiddly to leave as one-off shell snippets.

It wraps those workflows with:

- Safer defaults and clearer failure messages
- Consistent flags, JSON output, and help text
- Reusable checks before running system-changing commands
- One place to evolve personal Arch, macOS, Docker, AWS, and AI-tooling workflows

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

On macOS, choose the binary for your CPU:

```bash
# Apple Silicon
curl -L https://github.com/jyablonski/arc/releases/latest/download/arc-darwin-arm64 -o ~/.local/bin/arc
chmod +x ~/.local/bin/arc

# Intel
curl -L https://github.com/jyablonski/arc/releases/latest/download/arc-darwin-amd64 -o ~/.local/bin/arc
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

| Command         | Arch Linux                         | macOS                          | Example                        |
| --------------- | ---------------------------------- | ------------------------------ | ------------------------------ |
| `update system` | pacman, yay, paccache              | Homebrew                       | `arc update system`            |
| `update self`   | GitHub release binary              | GitHub release binary          | `arc update self`              |
| `update uv`     | `uv self update`                   | `uv self update`               | `arc update uv`                |
| `clean`         | pacman cache and orphans           | `brew cleanup` / `autoremove`  | `arc clean --orphans-only`     |
| `packages`      | pacman stats and size info         | Homebrew package summary       | `arc packages --json`          |
| `info`          | fastfetch                          | fastfetch                      | `arc info`                     |
| `parts`         | dmidecode, lspci, lshw             | system_profiler, sysctl        | `arc parts`                    |
| `installed`     | pacman explicit packages / AUR     | Homebrew formulae              | `arc installed --count`        |
| `sleep`         | systemd                            | pmset                          | `arc sleep`                    |
| `validate`      | Arch dependency checks             | macOS dependency checks        | `arc validate`                 |
| `setup`         | installs tools with pacman         | installs tools with Homebrew   | `arc setup`                    |
| `ai usage`      | AI coding tool usage               | AI coding tool usage           | `arc ai usage`                 |
| `skills`        | Shared AI/LLM skills               | Shared AI/LLM skills           | `arc skills sync`              |
| `rules`         | Shared AGENTS.md rules             | Shared AGENTS.md rules         | `arc rules sync`               |

Arch-only flags such as `--aur-only` and `--no-aur` return an unsupported-platform error on macOS when explicitly used.

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
│   ├── brew/
│   │   └── brew.go       # Homebrew-specific parsing and helpers
│   ├── pacman/
│   │   └── pacman.go     # pacman-specific parsing and helpers (+ kernel list)
│   ├── platform/         # typed platform detection
│   ├── pkgmgr/           # package-manager boundary (pacman/Homebrew)
│   ├── syscontrol/       # suspend / system-control boundary
│   ├── hardware/         # platform-specific hardware reporting
│   ├── setupdeps/        # platform-specific setup dependency installer
│   ├── deps/             # validate command dependency lists
│   ├── selfupdate/       # arc update self
│   ├── sysupdate/        # arc update system
│   ├── ghworkflow/       # arc gh restart-dashboard
│   ├── gitcleanup/       # arc git cleanup
│   └── skills/           # shared AI skills, provider paths, sync, validation
```

## License

MIT
