# Platform support

`arc` supports the two platforms this project is built around:

- Arch Linux, using pacman, yay, systemd, and Linux hardware tools.
- macOS, using Homebrew, `pmset`, `system_profiler`, and `sysctl`.

Most commands use the same command name on both platforms, but the underlying tool and some output differ.

## Commands shared by both platforms

These commands are available on Arch Linux and macOS:

| Area | Commands |
| --- | --- |
| System | `update system`, `update uv`, `clean`, `packages`, `installed`, `info`, `parts`, `sleep` |
| AI tooling | `ai usage`, `ai tokens`, `ai sessions`, `ai health`, `ai pricing` |
| Shared AI configuration | `skills`, `rules`, `mcp` |
| Local workflows | `setup`, `validate`, `stats`, `update self` |

The command name is shared, but the result is not always identical. For example, `packages` reports pacman data on Arch and Homebrew data on macOS, while `parts` reports native hardware diagnostics rather than a stable cross-platform schema.

## Arch Linux

`arc` expects an Arch-style package environment with pacman. AUR workflows use yay when it is installed.

- `arc update system` updates repository packages, reviews AUR changes, and then lets pacman or yay perform the update.
- `arc clean` clears the pacman cache, removes orphaned packages, and cleans `arc` update logs. Use `--logs-only` to remove only the logs.
- `arc packages` and `arc installed` read pacman package metadata.
- `arc sleep` uses `systemctl suspend`.
- `arc parts` uses tools such as `dmidecode`, `lspci`, `lshw`, and `nvidia-smi` when available.
- `arc setup` installs dependencies with pacman.

The flags `--no-aur`, `--yes`, and `--log` apply to the Linux `update system` workflow. The `installed --aur-only` filter is also Linux-only. See [Reviewing AUR diffs](aur_review.md) for the review process and warning signs.

## macOS

`arc` uses Homebrew for package operations. Homebrew must already be installed; `arc setup` installs required tools through Homebrew but does not install Homebrew itself.

- `arc update system` runs `brew update`, `brew upgrade`, and optional cleanup.
- `arc clean` runs Homebrew cleanup and autoremove, then cleans `arc` update logs. Use `--logs-only` to remove only the logs.
- `arc packages` and `arc installed` read Homebrew formula and cask metadata.
- `arc sleep` uses `pmset sleepnow`.
- `arc parts` uses `system_profiler` and `sysctl`.

Linux-only flags return an unsupported-platform error on macOS when explicitly used.

## Releases

GitHub Releases publish three binaries:

- `arc-linux-amd64`
- `arc-darwin-arm64`
- `arc-darwin-amd64`

macOS binaries are distributed directly from GitHub Releases rather than through a Homebrew formula. Unsigned binaries may be quarantined by macOS. See the [installation notes](../README.md#installation) if macOS blocks the first run.

## Development checks

The project tests Linux behavior on Ubuntu, runs macOS runtime tests, checks Darwin cross-compilation for both architectures, and builds all three release binaries in CI.

Run the regular tests and the cross-platform compile checks locally with:

```bash
go test ./...
GOOS=darwin GOARCH=arm64 go test -exec=true ./...
GOOS=darwin GOARCH=amd64 go test -exec=true ./...
```
