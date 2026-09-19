# AI usage

`arc ai usage` reports current usage windows for Claude Code, Codex, and Cursor. It reads credentials already stored by those tools, then calls their usage services. It does not log you in or write credentials.

Providers run in parallel, and one provider failing does not prevent the others from reporting. The command needs network access unless it can use its recent cache.

## Before you run it

Sign in with each tool you want to check:

| Tool | Local state used by `arc` | Setup |
| --- | --- | --- |
| Claude Code | OAuth credentials in `~/.claude/.credentials.json` or the macOS Keychain | Sign in with Claude Code using a supported subscription. |
| Codex | Credentials managed by the Codex CLI and its local app server | Run `codex login` first. |
| Cursor | The `cursorAuth/accessToken` value in Cursor's local state database | Sign in to Cursor. |

`arc` reads these credentials on demand. It does not copy them into the usage cache.

## Command

```bash
arc ai usage                         # all providers
arc ai usage --provider claude       # one provider
arc ai usage --provider claude,codex
arc ai usage --no-cache              # bypass the local cache
arc ai usage --short                 # one verdict line, for prompts and status bars
arc ai usage --json
```

Use `--provider` when you want to troubleshoot one tool. Selecting providers also bypasses the shared usage cache.

The human-readable output is one table across every provider. The bar fills with what has been **consumed**, the `left` column is what remains, and a window that has not started yet says so rather than showing a bare dash. Percentages are whole numbers, keeping one decimal only below 10% where the difference is actionable; a window with any consumption never reads as `100%`.

The closing line is the verdict — the one line worth reading on a return visit:

```
✓ all windows clear · tightest is claude 7 day at 97% left
```

A window below 20% remaining makes that line a warning, and an exhausted window makes it a failure. `--short` prints only that line.

Window names are normalized across providers (`5 hour`, `7 day`, `7 day · opus`, `month · api`). JSON keeps each provider's own wording in `label` and adds arc's name in `window`, along with provider-specific details that are not shown in the table.

## Cache

The cache is stored at `~/.cache/arc/ai-usage.json` on Linux and the platform's equivalent user cache directory elsewhere. It contains aggregated usage values, not credentials. Cached results are reused for 45 seconds during a normal all-provider run.

Use `--no-cache` for a fresh all-provider request. A provider-specific request is always live.

## Network and privacy

This command makes requests to provider usage services. It does not run login flows, upload session transcripts, or store access tokens in the cache. For a completely offline report, use [`arc ai tokens`](ai_tokens.md), [`arc ai sessions`](ai_sessions.md), or [`arc ai health`](ai_health.md).
