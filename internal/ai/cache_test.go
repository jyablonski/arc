package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	agg := AggregateReport{
		Providers: []ProviderResult{
			{Name: "claude", OK: true, Report: UsageReport{Windows: []UsageWindow{{Label: "5 hour", PercentUsed: 1}}}},
		},
	}
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, WriteCache(now, agg))

	got, ok, err := ReadCache(now.Add(10 * time.Second))
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Providers, 1)

	_, ok, err = ReadCache(now.Add(2 * time.Minute))
	require.NoError(t, err)
	require.False(t, ok)

	p := filepath.Join(tmp, "arc", "ai-usage.json")
	_, err = os.Stat(p)
	require.NoError(t, err)
}
