# arc

A personal CLI tool for system management and maintenance on Arch Linux.

## What It Does

`arc` consolidates common system tasks into a single command-line tool to provide a consistent interface for system operations with better argument handling, help text, colored output, and error handling.

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

## Commands

| Command     | Description                        | Example                        |
| ----------- | ---------------------------------- | ------------------------------ |
| `update`    | System updates via pacman and yay  | `arc update --no-aur`          |
| `clean`     | Remove cached packages and orphans | `arc clean --orphans-only`     |
| `packages`  | Package stats and size info        | `arc packages --top 10 --json` |
| `info`      | System information                 | `arc info --json`              |
| `parts`     | Hardware components                | `arc parts`                    |
| `installed` | List installed packages            | `arc installed --aur-only`     |
| `search`    | Search package repos               | `arc search neovim`            |
| `sleep`     | Suspend system                     | `arc sleep`                    |
| `validate`  | Check dependencies                 | `arc validate`                 |
| `setup`     | Install required tools             | `arc setup`                    |

Use `arc <command> --help` for detailed flag information.

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
│   └── pacman/
│       └── pacman.go     # pacman-specific parsing and helpers
```

## License

MIT