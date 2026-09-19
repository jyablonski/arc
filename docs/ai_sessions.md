# AI sessions

`arc ai sessions` lists recent Claude Code and Codex sessions from the same local logs used by [`arc ai tokens`](ai_tokens.md). It is offline, read-only, and does not upload transcripts.

Rows are led by age, which is also the sort key, then session ID, provider, model, message count, token count, project, and a title or first-prompt preview. The title fills the remaining terminal width. Cells with nothing to show are marked with `—` rather than left blank.

Automated sessions — Codex auto-review runs, which nobody resumes — are hidden by default and counted in the closing line, alongside how many sessions matched before `--limit` and the token total of what was shown:

```
8 of 20 sessions (--limit 0 for all) · 12 auto-review hidden (--all) · 87.1M tokens
```

## Command

```bash
arc ai sessions
arc ai sessions --limit 50
arc ai sessions --search checkout
arc ai sessions --provider claude
arc ai sessions --since 2026-01-01 --until 2026-01-31
arc ai sessions --resume
arc ai sessions --all                # include Codex auto-review sessions
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
| `--all` | `false` | Include automated sessions such as Codex auto-review. |
| `-j`, `--json` | `false` | Emit the session report as JSON. |

If no local session logs are available, the command reports no sessions. A failure in one provider does not block the other provider's results.
