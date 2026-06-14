package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestLayeredPricer_higherLayerWins(t *testing.T) {
	tokens := TokenBreakdown{Input: 1_000_000}
	p := LayeredPricer{Layers: []map[string]ModelPrice{
		{"claude-opus-4-6": {InputPerMillion: 99, OutputPerMillion: 1, Source: "override"}},
		{"claude-opus-4-6": {InputPerMillion: 5, OutputPerMillion: 25, Source: "litellm:anthropic"}},
		defaultModelPrices(),
	}}

	cost, source := p.Cost("claude-opus-4-6", tokens)
	require.InDelta(t, 99.0, cost, 1e-9)
	require.Equal(t, "override", source)
}

func TestLayeredPricer_fallsThroughToDefaults(t *testing.T) {
	tokens := TokenBreakdown{Input: 1_000_000}
	p := LayeredPricer{Layers: []map[string]ModelPrice{
		{"claude-fable-5": {InputPerMillion: 7, Source: "override"}},
		defaultModelPrices(),
	}}

	// A model only the defaults know about still prices from the bottom layer.
	cost, source := p.Cost("claude-opus-4-6", tokens)
	require.InDelta(t, 5.0, cost, 1e-9)
	require.Equal(t, "static-anthropic-api", source)
}

func TestLayeredPricer_unpriced(t *testing.T) {
	p := LayeredPricer{Layers: []map[string]ModelPrice{defaultModelPrices()}}
	cost, source := p.Cost("totally-unknown-model", TokenBreakdown{Input: 10})
	require.Zero(t, cost)
	require.Equal(t, "unpriced", source)
}

func TestNewLayeredPricer_layersCacheAndOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("HOME", tmp)

	require.NoError(t, WritePricingCache(time.Now(), "test", map[string]ModelPrice{
		"claude-fable-5": {InputPerMillion: 4, OutputPerMillion: 20, Source: "litellm:anthropic"},
	}))

	overridePath, err := PricingOverridePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(overridePath), filemode.Dir))
	require.NoError(t, os.WriteFile(overridePath, []byte(
		`{"claude-fable-5": {"input_per_million": 6, "output_per_million": 30}}`), 0o600))

	p := NewLayeredPricer()
	cost, source := p.Cost("claude-fable-5", TokenBreakdown{Input: 1_000_000})
	// override (6) beats the fetched cache (4).
	require.InDelta(t, 6.0, cost, 1e-9)
	require.Equal(t, "override", source)
}
