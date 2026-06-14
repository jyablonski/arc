# arc AI Pricing

`arc ai pricing` refreshes the local model pricing table that [`arc ai tokens`](ai_tokens.md) uses to turn token counts into API-equivalent cost estimates.

It exists so pricing can stay current **without an `arc` release**: when a new model ships, run this command (or hand-edit the override file) instead of waiting for a binary update.

This is the only networked command under `arc ai` besides [`arc ai usage`](ai_usage.md). `arc ai tokens` itself never makes network calls — it only reads whatever this command last cached.

## How pricing is resolved

Pricing is resolved from up to three layers, **highest priority first**. The first layer that knows a model wins:

| Priority | Layer             | File                            | Updated by       |
| -------- | ----------------- | ------------------------------- | ---------------- |
| 1        | User override     | `~/.config/arc/ai-pricing.json` | You, by hand     |
| 2        | Fetched cache     | `~/.cache/arc/ai-pricing.json`  | `arc ai pricing` |
| 3        | Built-in defaults | `internal/ai/pricing.go`        | An `arc` release |

- Each layer is a map of model ID → per-million rates; one rate per token type (input, output, cache_read, cache_write, reasoning).
- Model IDs are normalized (provider prefix and date suffix stripped, lowercased) and matched against each layer; unknown models are priced at `0` with source `unpriced`.
- Reasoning tokens fall back to the output rate when a model has no explicit reasoning rate.
- The `pricing_source` field on `arc ai tokens` output labels each group by the layer that priced it (`override`, `litellm:anthropic`, `litellm:openai`, `static-anthropic-api`, `static-openai-api`, `unpriced`, or `mixed` when a group spans more than one source).
- Reading prices is fully offline. Only `arc ai pricing` touches the network, and only when you run it.

## Command

```bash
arc ai pricing                 # fetch LiteLLM's table into ~/.cache/arc/ai-pricing.json
arc ai pricing --dry-run       # fetch and report counts without writing
arc ai pricing --source <url>  # use your own JSON in the same (LiteLLM) format
arc ai pricing -j              # emit the fetched table as JSON
```

### Flags

| Flag            | Values | Default         | Notes                                                                                  |
| --------------- | ------ | --------------- | -------------------------------------------------------------------------------------- |
| `--source`      | URL    | LiteLLM's table | Pricing table to fetch, in LiteLLM JSON format. Use your own hosted JSON if preferred. |
| `--dry-run`     | bool   | `false`         | Fetch and report model counts without writing the cache.                               |
| `-j` / `--json` | bool   | `false`         | Emit the fetched table (source, counts, new models, prices) as JSON.                   |

The default source is [LiteLLM's community pricing table](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json); only its `anthropic` and `openai` entries are kept. Per-token costs are converted to per-million on the way in.

## Overriding pricing

For a model the fetched source does not list yet — or to correct a rate — add it to the override file. It takes effect immediately and survives future refreshes, since the override layer always wins:

```jsonc
// ~/.config/arc/ai-pricing.json
{
  "claude-fable-5": {
    "input_per_million": 5,
    "output_per_million": 25,
    "cache_read_per_million": 0.5,
    "cache_write_per_million": 6.25
  }
}
```

Keys are model IDs (normalized the same way as lookups). Any rate you omit defaults to `0`; an omitted `reasoning_per_million` falls back to the output rate. Entries without a `source` are labelled `override` in output.

> Costs remain an API-equivalent approximation. The override layer is authoritative when present, so keep it accurate.
