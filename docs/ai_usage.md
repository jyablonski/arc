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
arc ai usage --json
```

Use `--provider` when you want to troubleshoot one tool. Selecting providers also bypasses the shared usage cache.

The human-readable output shows each provider's usage window, remaining percentage, and reset time. JSON output includes provider-specific details that are not shown in the table.

## Cache

The cache is stored at `~/.cache/arc/ai-usage.json` on Linux and the platform's equivalent user cache directory elsewhere. It contains aggregated usage values, not credentials. Cached results are reused for 45 seconds during a normal all-provider run.

Use `--no-cache` for a fresh all-provider request. A provider-specific request is always live.

## Network and privacy

This command makes requests to provider usage services. It does not run login flows, upload session transcripts, or store access tokens in the cache. For a completely offline report, use [`arc ai tokens`](ai_tokens.md), [`arc ai sessions`](ai_sessions.md), or [`arc ai health`](ai_health.md).
