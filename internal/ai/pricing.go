package ai

import "strings"

type Pricer interface {
	Cost(model string, tokens TokenBreakdown) (float64, string)
}

type ModelPrice struct {
	InputPerMillion      float64
	OutputPerMillion     float64
	CacheReadPerMillion  float64
	CacheWritePerMillion float64
	ReasoningPerMillion  float64
	Source               string
}

type StaticPricer struct {
	Prices map[string]ModelPrice
}

func NewStaticPricer() StaticPricer {
	return StaticPricer{Prices: defaultModelPrices()}
}

func (p StaticPricer) Cost(model string, tokens TokenBreakdown) (float64, string) {
	price, ok := p.lookup(model)
	if !ok {
		return 0, "unpriced"
	}
	outputRate := price.OutputPerMillion
	reasoningRate := price.ReasoningPerMillion
	if reasoningRate == 0 {
		reasoningRate = outputRate
	}
	cost := perMillion(tokens.Input, price.InputPerMillion) +
		perMillion(tokens.Output, outputRate) +
		perMillion(tokens.CacheRead, price.CacheReadPerMillion) +
		perMillion(tokens.CacheWrite, price.CacheWritePerMillion) +
		perMillion(tokens.Reasoning, reasoningRate)
	return cost, price.Source
}

func (p StaticPricer) lookup(model string) (ModelPrice, bool) {
	normalized := normalizeModelID(model)
	if price, ok := p.Prices[normalized]; ok {
		return price, true
	}
	for key, price := range p.Prices {
		if strings.Contains(normalized, key) {
			return price, true
		}
	}
	return ModelPrice{}, false
}

func perMillion(tokens int64, rate float64) float64 {
	if tokens <= 0 || rate <= 0 {
		return 0
	}
	return float64(tokens) * rate / 1_000_000
}

func normalizeModelID(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(m, "/"); idx >= 0 && idx < len(m)-1 {
		m = m[idx+1:]
	}
	for _, suffix := range []string{"-20250514", "-20250219", "-20241022", "-20240620"} {
		m = strings.TrimSuffix(m, suffix)
	}
	return strings.ReplaceAll(m, "_", "-")
}

func defaultModelPrices() map[string]ModelPrice {
	// TODO: Pull provider pricing from an external source so API-equivalent
	// costs can stay current without requiring an arc release.
	openai125 := ModelPrice{
		InputPerMillion:     1.25,
		CacheReadPerMillion: 0.125,
		OutputPerMillion:    10,
		ReasoningPerMillion: 10,
		Source:              "static-openai-api",
	}
	openai175 := ModelPrice{
		InputPerMillion:     1.75,
		CacheReadPerMillion: 0.175,
		OutputPerMillion:    14,
		ReasoningPerMillion: 14,
		Source:              "static-openai-api",
	}
	openai55 := ModelPrice{
		InputPerMillion:     5,
		CacheReadPerMillion: 0.50,
		OutputPerMillion:    30,
		ReasoningPerMillion: 30,
		Source:              "static-openai-api",
	}
	sonnet := ModelPrice{
		InputPerMillion:      3,
		CacheReadPerMillion:  0.30,
		CacheWritePerMillion: 3.75,
		OutputPerMillion:     15,
		Source:               "static-anthropic-api",
	}
	opus := ModelPrice{
		InputPerMillion:      5,
		CacheReadPerMillion:  0.50,
		CacheWritePerMillion: 6.25,
		OutputPerMillion:     25,
		Source:               "static-anthropic-api",
	}
	haiku := ModelPrice{
		InputPerMillion:      1,
		CacheReadPerMillion:  0.10,
		CacheWritePerMillion: 1.25,
		OutputPerMillion:     5,
		Source:               "static-anthropic-api",
	}
	return map[string]ModelPrice{
		"gpt-5":               openai125,
		"gpt-5.5":             openai55,
		"gpt-5.5-chat-latest": openai55,
		"gpt-5-codex":         openai125,
		"gpt-5.1-codex":       openai125,
		"gpt-5.1-codex-max":   openai125,
		"gpt-5.2-codex":       openai175,
		"gpt-5.2-chat-latest": openai175,
		"gpt-5.2":             openai175,
		"claude-sonnet-4":     sonnet,
		"claude-sonnet-4.5":   sonnet,
		"claude-sonnet-4-5":   sonnet,
		"claude-sonnet-4.6":   sonnet,
		"claude-sonnet-4-6":   sonnet,
		"claude-opus-4":       opus,
		"claude-opus-4.5":     opus,
		"claude-opus-4-5":     opus,
		"claude-opus-4.6":     opus,
		"claude-opus-4-6":     opus,
		"claude-haiku-4.5":    haiku,
		"claude-haiku-4-5":    haiku,
	}
}
