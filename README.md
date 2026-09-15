# arc

A personal CLI for maintaining Arch Linux and macOS machines and managing local AI-tool workflows.

`arc` gives common maintenance, inspection, and AI-tooling tasks one consistent command-line interface. It wraps the tools already used by each platform: pacman and yay on Arch Linux, and Homebrew on macOS. It adds clearer output, safer workflows, and JSON output for scripts.

## What it does

- Maintains the system: update packages, clean caches and unused dependencies, inspect hardware, list installed packages, and suspend the machine.
- Tracks AI-tool usage: fetch current usage for Claude, Codex, and Cursor, or inspect local Claude Code and Codex session history with an API-equivalent cost estimate.
- Shares AI configuration: keep skills, `AGENTS.md` rules, and MCP configuration in canonical locations and sync them across Claude, Codex, Cursor, and opencode.
- Handles a few recurring workflows: validate dependencies, update `arc`, clean Docker resources, clean Git repositories, inspect AWS identity, send incident alerts, and view local `arc` usage statistics.

`arc` is designed for personal, local use. It does not replace the tools it wraps, and it does not create or run MCP servers.

## Quick start

```bash
arc validate
arc info
arc packages --top 25
arc ai health
arc ai tokens
```

Most commands print human-readable output. Add `--json` or `-j` when the result needs to be consumed by another tool.

## Installation

Release binaries are available for Linux x86_64, macOS Intel, and Apple Silicon Macs.

```bash
mkdir -p ~/.local/bin

# Choose the asset for your system:
release_asset=arc-linux-amd64       # Linux x86_64
# release_asset=arc-darwin-amd64    # macOS Intel
# release_asset=arc-darwin-arm64    # macOS Apple Silicon

curl -L "https://github.com/jyablonski/arc/releases/latest/download/$release_asset" -o ~/.local/bin/arc

chmod +x ~/.local/bin/arc
export PATH="$HOME/.local/bin:$PATH"
```

If macOS blocks an unsigned release binary on its first launch, approve it in System Settings or remove its quarantine attribute:

```bash
xattr -d com.apple.quarantine ~/.local/bin/arc
```

To build and install from source:

```bash
git clone https://github.com/jyablonski/arc.git
cd arc
make install
```

Once `arc` is installed, update it with:

```bash
arc update self
```

## Common commands

### System maintenance

| Command | Purpose | Example |
| --- | --- | --- |
| `update system` | Update system packages; includes AUR review on Arch | `arc update system` |
| `update uv` | Update the `uv` package manager | `arc update uv` |
| `clean` | Clean package caches, unused dependencies, and `arc` logs | `arc clean` |
| `packages` | Show package counts, sizes, and cache information | `arc packages --top 25` |
| `installed` | List explicitly installed packages | `arc installed` |
| `info` | Show system information with `fastfetch` | `arc info` |
| `parts` | Show hardware information | `arc parts` |
| `sleep` | Suspend the machine | `arc sleep` |

Arch uses pacman and yay; macOS uses Homebrew. Some output is necessarily platform-specific. Arch-only flags such as `--aur-only` and `--no-aur` return an error on macOS when used explicitly.

### AI tooling

| Command | Purpose | Example |
| --- | --- | --- |
| `ai usage` | Fetch current usage limits for Claude, Codex, and Cursor | `arc ai usage` |
| `ai tokens` | Report historical local token usage and estimated cost | `arc ai tokens` |
| `ai sessions` | List recent local Claude Code and Codex sessions | `arc ai sessions` |
| `ai health` | Check AI-tool auth, dependencies, and local configuration | `arc ai health` |
| `ai pricing` | Refresh the local pricing cache used by `ai tokens` | `arc ai pricing` |

`ai usage` uses credentials already stored by the vendor tools and calls their usage APIs. `ai tokens` and `ai sessions` read local session data; they do not upload it.

### Shared AI configuration

| Command | Purpose | Canonical location |
| --- | --- | --- |
| `skills` | Manage shared skill definitions | `~/ai/skills/` |
| `rules` | Manage shared `AGENTS.md` rules | `~/ai/AGENTS.md` |
| `mcp` | Manage canonical MCP configuration and sync it across AI tools | `~/ai/mcp.json` |

Skills and rules use symlinks where the provider supports them. MCP configuration is merged into each provider's existing configuration format, so unrelated settings and hand-managed entries are preserved. `arc mcp add` writes an MCP configuration entry; it does not install, create, or start an MCP server. See [MCP configuration](docs/mcp.md).

Useful examples:

```bash
arc skills sync
arc rules sync
arc mcp list
arc mcp sync
```

### Other workflows

| Command | Purpose | Example |
| --- | --- | --- |
| `setup` | Install tools required by `arc` | `arc setup` |
| `validate` | Check required tools in `PATH` | `arc validate` |
| `docker clean` | Prune Docker images, containers, and volumes | `arc docker clean` |
| `git cleanup` | Remove merged branches and prune remote references | `arc git cleanup` |
| `aws whoami` | Show the current AWS identity | `arc aws whoami` |
| `incident [title]` | Send an incident alert to Slack and optionally Discord | `arc incident "Database outage"` |
| `stats` | Show local `arc` command usage | `arc stats --json` |

Run `arc <command> --help` for flags and detailed behavior.

## Documentation

See the [docs](docs/) folder for detailed documentation on each command and workflow.

## License

MIT
