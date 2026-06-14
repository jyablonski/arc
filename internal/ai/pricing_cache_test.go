package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestPricingCacheRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	prices := map[string]ModelPrice{
		"claude-sonnet-4-5": {InputPerMillion: 3, OutputPerMillion: 15, Source: "litellm:anthropic"},
	}
	require.NoError(t, WritePricingCache(now, "test-source", prices))

	cf, ok, err := ReadPricingCacheFile()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "test-source", cf.Source)
	require.True(t, now.Equal(cf.FetchedAt))

	got, ok := ReadPricingCache()
	require.True(t, ok)
	require.InDelta(t, 3.0, got["claude-sonnet-4-5"].InputPerMillion, 1e-9)
}

func TestReadPricingCache_missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	_, ok := ReadPricingCache()
	require.False(t, ok)
}

func TestReadPricingOverride_normalizesAndLabels(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	path, err := PricingOverridePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), filemode.Dir))
	require.NoError(t, os.WriteFile(path, []byte(`{
		"claude-fable-5": {"input_per_million": 5, "output_per_million": 25},
		"my-org/Custom-Model": {"input_per_million": 1, "output_per_million": 2, "source": "internal"}
	}`), 0o600))

	override, ok := ReadPricingOverride()
	require.True(t, ok)

	fable := override["claude-fable-5"]
	require.InDelta(t, 5.0, fable.InputPerMillion, 1e-9)
	require.Equal(t, "override", fable.Source) // defaulted when blank

	// provider prefix is stripped and id lowercased by normalizeModelID.
	custom, ok := override["custom-model"]
	require.True(t, ok)
	require.Equal(t, "internal", custom.Source) // explicit source preserved
}
