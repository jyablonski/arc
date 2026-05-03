# Shared AI Rules

`arc rules` keeps one shared rules file linked into supported AI provider locations.

The canonical file is:

```text
~/ai/AGENTS.md
```

Provider targets include:

| Provider | Target |
| -------- | ------ |
| Claude | `~/.claude/CLAUDE.md` |
| Codex | `~/.codex/AGENTS.md` |
| opencode | `~/.config/opencode/AGENTS.md` |

`arc` creates parent directories as needed and links provider rule files back to the canonical file.

## Commands

Ensure provider symlinks are in place:

```bash
arc rules sync
```

Show per-provider symlink status:

```bash
arc rules status
```
