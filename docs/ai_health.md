# AI health

`arc ai health` runs offline checks across the local AI toolchain. It does not make network requests, refresh tokens, or modify configuration.

## What it checks

- Claude Code, Codex, and Cursor authentication state, including token expiry when it can be determined.
- Claude Code and Codex command-line tools on `PATH`.
- The local pricing cache used by `arc ai tokens`.
- Shared skills and `AGENTS.md` rules when those canonical files exist.
- Shared MCP configuration status when `~/ai/mcp.json` exists.

Checks are reported as `ok`, `warn`, or `fail`. Warnings do not affect the exit status. The command exits non-zero when a check fails.

## Command

```bash
arc ai health
arc ai health --provider claude
arc ai health --provider codex,cursor
arc ai health --json
```

When `--provider` is provided, the report is limited to that provider. Machine-wide checks for pricing and shared configuration are included only in the normal all-provider run.

## Common fixes

| Check | Fix |
| --- | --- |
| Claude authentication | Sign in with Claude Code. |
| Codex authentication | Run `codex login`. |
| Cursor authentication | Open Cursor and sign in. |
| Missing CLI | Run `arc setup`, or install the tool manually. |
| Stale pricing | Run `arc ai pricing`. |
| Skills or rules out of sync | Run `arc skills sync` or `arc rules sync`. |
| MCP drift or conflicts | Run `arc mcp list`, then `arc mcp sync` after reviewing the differences. |

`arc ai health` checks local state only. A successful authentication check does not guarantee that a provider's live usage service is reachable; use [`arc ai usage`](ai_usage.md) for that.
