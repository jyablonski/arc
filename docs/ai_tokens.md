# arc AI Tokens

`arc ai tokens` scans local Claude Code and Codex CLI session logs and reports historical token usage with an API-equivalent cost estimate based on built-in model pricing.

This is historical local usage — what your installed tools actually logged on disk — not subscription quota. For live, provider-reported quota windows use [`arc ai usage`](ai_usage.md) instead.

No network: unlike `arc ai usage`, this command makes zero outward calls. It only reads JSONL session files already on disk and prices them locally.

## Sources

| Provider | Local source                                                   | Notes                                                                                   |
| -------- | -------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Claude   | `~/.claude/projects/` and `~/.claude/transcripts/` JSONL files | Token counts parsed per message.                                                        |
| Codex    | `~/.codex/sessions/` JSONL `token_count` events                | Reasoning tokens reported separately when present.                                      |
| Cursor   | *Not supported here*                                           | Cursor does not expose local token transcripts; use `arc ai usage` for its quota/spend. |

Each record carries a token breakdown of input, output, cache_read, cache_write, and reasoning tokens. Providers are read independently — one failing does not block the others.

## Pricing

Cost is an API-equivalent estimate: it answers "what would these tokens have cost at pay-as-you-go API rates," not what you were actually billed under a subscription.

- Pricing is a static, built-in table (`internal/ai/pricing.go`), one rate per million tokens for each token type.
- Model IDs are normalized (provider prefix and date suffix stripped) and matched against the table; unknown models are priced at `0` with source `unpriced`.
- Reasoning tokens fall back to the output rate when a model has no explicit reasoning rate.
- The `pricing_source` field labels each group (`static-anthropic-api`, `static-openai-api`, `unpriced`, or `mixed` when a group spans more than one source).

> Pricing is baked into the binary, so estimates only update when `arc` does. Treat the cost column as an approximation.

## Command

```bash
arc ai tokens                                   # group by provider,model (default)
arc ai tokens -j                                # JSON
arc ai tokens --provider claude                 # one provider (claude or codex)
arc ai tokens --since 2026-01-01 --until 2026-01-31
arc ai tokens --group-by date
arc ai tokens --group-by session,model --sort-by tokens
arc ai tokens --show-total-tokens
```

### Flags

| Flag                  | Values                                                         | Default          | Notes                                                                                             |
| --------------------- | -------------------------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------- |
| `--provider`          | comma-separated: `claude`, `codex`                             | all              | Filters which local logs are scanned.                                                             |
| `--since`             | `YYYY-MM-DD` or RFC3339                                        | unbounded        | Keep records on or after this date.                                                               |
| `--until`             | `YYYY-MM-DD` or RFC3339                                        | unbounded        | Keep records on or before this date. Must be on or after `--since`.                               |
| `--group-by`          | `provider`, `model`, `provider,model`, `date`, `session,model` | `provider,model` | How rows are aggregated.                                                                          |
| `--sort-by`           | `cost`, `tokens`, `date`, `group`                              | `cost` desc      | `date` asc when `--group-by date`.                                                                |
| `--sort-order`        | `asc`, `desc`                                                  | `desc`           | `asc` when sorting by `date` under `--group-by date`.                                             |
| `--show-total-tokens` | bool                                                           | `false`          | Adds the aggregate token-total column; hidden by default because cache reads dominate raw totals. |
| `-j` / `--json`       | bool                                                           | `false`          | Emits the full `HistoryReport` (per-provider results, groups, and totals).                        |

- Exit code is non-zero only if every selected provider fails.
- The default table hides the raw token total: cache-read tokens are cheap but voluminous, so summed totals are misleading. The cost column is the more meaningful comparison; use `--show-total-tokens` if you want the raw number anyway.
