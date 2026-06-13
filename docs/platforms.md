# Platform Support

`arc` supports the two platforms this tool is built around:

- Arch Linux, using pacman, yay, systemd, and Linux hardware tools
- macOS, using Homebrew, pmset, system_profiler, and sysctl

The command tree stays mostly the same on both platforms. Commands dispatch to platform-specific implementations internally, so users can generally run the same `arc` command and get the native behavior for that system.

## How It Works

Platform support is selected once when the CLI starts. The `cmd` package builds an `App` value from `platform.Detect()`:

```go
var app = newApp(platform.Detect())
```

`internal/platform` exposes a small typed platform enum:

- `platform.Linux`
- `platform.Darwin`
- `platform.Unknown`

The app wires platform-specific implementations behind narrow interfaces:

- `internal/pkgmgr` handles package operations like update, clean, installed packages, and package summaries.
- `internal/syscontrol` handles system actions like sleep.
- `internal/hardware` handles hardware reporting.
- `internal/setupdeps` handles dependency installation.
- `internal/deps` provides the tool list used by `arc validate`.

Most command handlers do not switch on `runtime.GOOS`. They call the app dependency for that concern:

```go
return app.PkgMgr.Clean(pkgmgr.CleanOptions{
    OrphansOnly: cleanOrphansOnly,
    CacheOnly:   cleanCacheOnly,
})
```

The platform switch lives inside the boundary factory instead:

```go
func New(os platform.OS) Manager {
    switch os {
    case platform.Linux:
        return linuxManager{}
    case platform.Darwin:
        return darwinManager{}
    default:
        return unsupportedManager{}
    }
}
```

This keeps commands mostly platform-blind while still allowing each platform implementation to use its native tools. The main exception is platform-specific user intent, such as `--aur-only` or `--no-aur`, where the command returns a clear error if the flag is used on macOS.

## Shared Commands

These commands are intended to work on both Arch Linux and macOS:

- `arc update self`
- `arc update uv`
- `arc update system`
- `arc clean`
- `arc packages`
- `arc installed`
- `arc info`
- `arc parts`
- `arc sleep`
- `arc validate`
- `arc setup`
- `arc ai usage`
- `arc skills`
- `arc rules`

Some output is intentionally platform-specific. For example, `arc packages` reports pacman package details on Arch and Homebrew package details on macOS. `arc parts` is human-readable hardware diagnostics, not a stable cross-platform schema.

## Arch Linux Behavior

On Arch Linux:

- `arc update system` runs the pacman/yay update workflow. The yay AUR step is interactive and forces diff and PKGBUILD review prompts before building. Before yay runs, arc triages pending AUR updates against the AUR RPC: it flags maintainer changes (baseline kept at `~/.local/state/arc/aur-provenance.json`), orphaned packages, and high-signal PKGBUILD patterns. Packages in `IgnorePkg` are skipped (arc won't review what yay won't upgrade), and it stays quiet when nothing is pending. It only surfaces findings — you still decide at the diffmenu.
- `arc clean` uses pacman cache and orphan cleanup.
- `arc packages` and `arc installed` read pacman package metadata.
- `arc sleep` uses systemd.
- `arc parts` uses tools like `dmidecode`, `lspci`, `lshw`, and `nvidia-smi`.
- `arc setup` installs dependencies with pacman.

Arch-only flags such as `--aur-only` and `--no-aur` are valid on Arch Linux.

## macOS Behavior

On macOS:

- `arc update system` runs `brew update`, `brew upgrade`, and optional `brew cleanup`.
- `arc clean` uses `brew cleanup` and `brew autoremove`.
- `arc packages` and `arc installed` read Homebrew formula and cask metadata.
- `arc sleep` uses `pmset sleepnow`.
- `arc parts` uses `system_profiler` and `sysctl`.
- `arc setup` installs missing tools with Homebrew, but it does not install Homebrew itself.

Arch-only flags return an unsupported-platform error on macOS when explicitly used.

## Releases

GitHub releases publish three binaries:

- `arc-linux-amd64`
- `arc-darwin-arm64`
- `arc-darwin-amd64`

macOS binaries are distributed directly from GitHub Releases, not through a Homebrew formula. Unsigned release binaries may be quarantined by macOS; see the README install notes for the `xattr` command if macOS blocks the first run.

## Testing

The CI pipeline verifies platform support with:

- Linux tests and coverage on Ubuntu
- macOS runtime tests on `macos-latest`
- Darwin cross-compile checks for `arm64` and `amd64`
- release builds for Linux and both macOS architectures

For local checks:

```bash
go test ./...
GOOS=darwin GOARCH=arm64 go test -exec=true ./...
GOOS=darwin GOARCH=amd64 go test -exec=true ./...
```
