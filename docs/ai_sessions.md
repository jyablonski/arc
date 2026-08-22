# AI sessions

`arc ai sessions` lists recent Claude Code and Codex sessions from the same local logs used by [`arc ai tokens`](ai_tokens.md). It is offline, read-only, and does not upload transcripts.

Each row includes the provider, session ID, age, model, message count, token count, project, and a title or first-prompt preview.

## Command

```bash
arc ai sessions
arc ai sessions --limit 50
arc ai sessions --search checkout
arc ai sessions --provider claude
arc ai sessions --since 2026-01-01 --until 2026-01-31
arc ai sessions --resume
arc ai sessions --json
```

`--resume` prints the command that reopens each session in its original tool. Copy the command you want to run; `arc` does not reopen sessions automatically.

## Flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `--provider` | all | Limit results to `claude`, `codex`, or both. |
| `--since` | unbounded | Include sessions active on or after this date. |
| `--until` | unbounded | Include sessions started on or before this date. |
| `--limit` | `20` | Show at most this many sessions. Use `0` for no limit. |
| `--search` | empty | Match text in the project, title, or session ID. |
| `--resume` | `false` | Print a provider-specific resume command. |
| `-j`, `--json` | `false` | Emit the session report as JSON. |

If no local session logs are available, the command reports no sessions. A failure in one provider does not block the other provider's results.
