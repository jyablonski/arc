# AI token history

`arc ai tokens` reads local Claude Code and Codex CLI session logs and reports historical token usage with an API-equivalent cost estimate. It is not a subscription-quota report. Use [`arc ai usage`](ai_usage.md) for live provider usage.

The command is offline. It reads JSONL files already on disk and uses the local pricing layers described in [AI pricing](ai_pricing.md). It does not upload session data.

## Sources

| Provider | Local source | Notes |
| --- | --- | --- |
| Claude Code | `~/.claude/projects/` and `~/.claude/transcripts/` | Token counts are parsed from message records. |
| Codex CLI | `~/.codex/sessions/` | `token_count` events include reasoning tokens when available. |
| Cursor | Not supported | Cursor does not expose local token transcripts. Use [`arc ai usage`](ai_usage.md) for its quota or spend data. |

Providers are read independently. A failure in one provider does not block the others.

## Cost estimates

The cost is an API-equivalent estimate. It answers what the recorded tokens would cost at pay-as-you-go API rates, not what a subscription billed you.

Rates are resolved in this order: the hand-edited override, the fetched cache, and built-in defaults. Unknown models are reported with a cost of `0` and a pricing source of `unpriced`. See [AI pricing](ai_pricing.md) to refresh or override rates.

## Command

```bash
arc ai tokens
arc ai tokens --json
arc ai tokens --provider claude
arc ai tokens --since 2026-01-01 --until 2026-01-31
arc ai tokens --group-by date
arc ai tokens --group-by session,model --sort-by tokens
arc ai tokens --show-total-tokens
```

The default table groups by provider and model. It hides the raw token total because cache-read tokens can dominate the count while contributing relatively little cost. Use `--show-total-tokens` when the raw count is useful.

## Flags

| Flag | Values | Default | Purpose |
| --- | --- | --- | --- |
| `--provider` | `claude`, `codex`, or a comma-separated list | all | Limit which logs are scanned. |
| `--since` | `YYYY-MM-DD` or RFC3339 | unbounded | Include records on or after this date. |
| `--until` | `YYYY-MM-DD` or RFC3339 | unbounded | Include records on or before this date. |
| `--group-by` | `provider`, `model`, `provider,model`, `date`, `session,model` | `provider,model` | Choose the aggregation. |
| `--sort-by` | `cluster`, `cost`, `tokens`, `date`, `group` | `cluster` descending | Choose the row order. |
| `--sort-order` | `asc`, `desc` | `desc` | Override the sort direction. |
| `--show-total-tokens` | bool | `false` | Add the raw aggregate token column. |
| `-j`, `--json` | bool | `false` | Emit the full report as JSON. |

The command exits non-zero only when every selected provider fails.

For individual session summaries and resume commands, see [AI sessions](ai_sessions.md).
