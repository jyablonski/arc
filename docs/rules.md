# Shared AI rules

`arc rules` keeps one canonical `AGENTS.md` file and links it into the supported AI tools. The canonical file is:

```text
~/ai/AGENTS.md
```

The current provider targets are:

| Provider | Target |
| --- | --- |
| Claude | `~/.claude/CLAUDE.md` |
| Codex | `~/.codex/AGENTS.md` |
| opencode | `~/.config/opencode/AGENTS.md` |

Cursor does not currently have a rules-file target supported by `arc`.

`arc` creates parent directories as needed. If a provider target contains real content instead of an `arc` symlink, sync reports a conflict and leaves it alone.

## Commands

```bash
arc rules sync
arc rules sync --dry-run
arc rules status
arc rules status --json
```

Sync is one-way. Edit `~/ai/AGENTS.md`, then run `arc rules sync` to update provider links.
