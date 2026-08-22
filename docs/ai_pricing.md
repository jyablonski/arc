# AI pricing

`arc ai pricing` refreshes the local model pricing table used by [`arc ai tokens`](ai_tokens.md). It lets pricing change without waiting for a new `arc` release.

This is the only networked command under `arc ai` besides [`arc ai usage`](ai_usage.md). Token history itself stays offline and reads the last pricing data available locally.

## How pricing is resolved

The first layer that contains a model wins:

| Priority | Layer | File | Updated by |
| --- | --- | --- | --- |
| 1 | User override | `~/.config/arc/ai-pricing.json` | You, by hand |
| 2 | Fetched cache | `~/.cache/arc/ai-pricing.json` | `arc ai pricing` |
| 3 | Built-in defaults | `internal/ai/pricing.go` | An `arc` release |

Each entry stores per-million-token rates for input, output, cache reads, cache writes, and reasoning. Model IDs are normalized before lookup. Unknown models cost `0` and are labeled `unpriced`. If reasoning has no separate rate, the output rate is used.

The `pricing_source` field in `arc ai tokens --json` identifies the layer that supplied each rate. A group can be `mixed` when it contains more than one source.

## Refresh the cache

```bash
arc ai pricing
arc ai pricing --dry-run
arc ai pricing --source <url>
arc ai pricing --json
```

The default source is [LiteLLM's community pricing table](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json). `arc` keeps its Anthropic and OpenAI entries and converts the rates to per-million-token values.

| Flag | Purpose |
| --- | --- |
| `--source <url>` | Fetch a JSON pricing table in LiteLLM format from another URL. |
| `--dry-run` | Fetch and report counts without writing the cache. |
| `-j`, `--json` | Emit the fetched table and summary as JSON. |

## Override a model

Add a model to `~/.config/arc/ai-pricing.json` when the fetched table does not include it or when you need to correct a rate:

```jsonc
{
  "claude-fable-5": {
    "input_per_million": 5,
    "output_per_million": 25,
    "cache_read_per_million": 0.5,
    "cache_write_per_million": 6.25
  }
}
```

Keys use the same normalized model IDs as lookups. Omitted rates default to `0`, and an omitted reasoning rate falls back to the output rate. Overrides always win and remain in effect after future cache refreshes.

These figures are estimates for API-equivalent usage. They are not a statement of subscription billing.
