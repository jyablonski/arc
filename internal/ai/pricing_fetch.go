package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jyablonski/arc/internal/boundary"
)

// DefaultPricingSource is LiteLLM's community-maintained model pricing table.
// It is the default fetched by `arc ai tokens pricing`; override with --source.
const DefaultPricingSource = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// pricingProviders limits the fetched table to the providers arc reports on, so
// the cache stays small and lookups don't collide with unrelated model names.
var pricingProviders = map[string]bool{
	"anthropic": true,
	"openai":    true,
}

// litellmEntry is the subset of each LiteLLM record arc consumes. Costs are
// per-token; arc stores per-million, so values are scaled on conversion.
type litellmEntry struct {
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	LiteLLMProvider             string  `json:"litellm_provider"`
}

// FetchPricing downloads a LiteLLM-format pricing table from source and converts
// it to arc's per-million ModelPrice map, keyed by normalized model ID. Only
// anthropic/openai chat entries with a usable input cost are kept. The client
// seam (boundary.HTTPDoer) makes this unit-testable with httptest.
func FetchPricing(ctx context.Context, source string, client boundary.HTTPDoer) (map[string]ModelPrice, error) {
	if strings.TrimSpace(source) == "" {
		source = DefaultPricingSource
	}
	if client == nil {
		return nil, fmt.Errorf("nil http client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", source, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("pricing source %s returned %s", source, resp.Status)
	}

	var raw map[string]litellmEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode pricing table: %w", err)
	}

	out := make(map[string]ModelPrice)
	for model, entry := range raw {
		provider := strings.ToLower(strings.TrimSpace(entry.LiteLLMProvider))
		if !pricingProviders[provider] {
			continue
		}
		if entry.InputCostPerToken <= 0 && entry.OutputCostPerToken <= 0 {
			continue
		}
		key := normalizeModelID(model)
		if key == "" {
			continue
		}
		out[key] = ModelPrice{
			InputPerMillion:      entry.InputCostPerToken * 1_000_000,
			OutputPerMillion:     entry.OutputCostPerToken * 1_000_000,
			CacheReadPerMillion:  entry.CacheReadInputTokenCost * 1_000_000,
			CacheWritePerMillion: entry.CacheCreationInputTokenCost * 1_000_000,
			Source:               "litellm:" + provider,
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pricing source %s yielded no usable anthropic/openai entries", source)
	}
	return out, nil
}
