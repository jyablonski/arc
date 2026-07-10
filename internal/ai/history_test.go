package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// defaultPricer prices from the built-in table only, keeping tests hermetic
// (NewLayeredPricer would also read the user's override and cache files).
func defaultPricer() LayeredPricer {
	return LayeredPricer{Layers: []map[string]ModelPrice{defaultModelPrices()}}
}

func TestDefaultPricer_Cost(t *testing.T) {
	cost, source := defaultPricer().Cost("gpt-5-codex", TokenBreakdown{
		Input:     1_000_000,
		CacheRead: 1_000_000,
		Output:    1_000_000,
		Reasoning: 1_000_000,
	})
	require.Equal(t, "static-openai-api", source)
	require.InDelta(t, 21.375, cost, 1e-9)

	cost, source = defaultPricer().Cost("gpt-5.5", TokenBreakdown{Input: 1_000_000})
	require.Equal(t, "static-openai-api", source)
	require.InDelta(t, 5, cost, 1e-9)
}

func TestGroupTokenRecords_providerModel(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	groups := GroupTokenRecords([]TokenRecord{
		{Provider: "codex", Model: "gpt-5", Timestamp: ts, Tokens: TokenBreakdown{Input: 10}, CostUSD: 0.1, PricingSource: "a"},
		{Provider: "codex", Model: "gpt-5", Timestamp: ts, Tokens: TokenBreakdown{Output: 20}, CostUSD: 0.2, PricingSource: "a"},
	}, "provider,model")
	require.Len(t, groups, 1)
	require.Equal(t, "codex", groups[0].Provider)
	require.Equal(t, "gpt-5", groups[0].Model)
	require.Equal(t, int64(10), groups[0].Tokens.Input)
	require.Equal(t, int64(20), groups[0].Tokens.Output)
	require.InDelta(t, 0.3, groups[0].CostUSD, 1e-9)
	require.Equal(t, 2, groups[0].Records)
}

func TestNormalizeHistorySort_defaultsToClusterExceptDate(t *testing.T) {
	sortBy, sortOrder := NormalizeHistorySort("provider,model", "", "")
	require.Equal(t, "cluster", sortBy)
	require.Equal(t, "desc", sortOrder)

	sortBy, sortOrder = NormalizeHistorySort("date", "", "")
	require.Equal(t, "date", sortBy)
	require.Equal(t, "asc", sortOrder)
}

func TestSortUsageGroups_clusterBandsByProvider(t *testing.T) {
	groups := []UsageGroup{
		{Provider: "codex", Model: "gpt-5.5", CostUSD: 386},
		{Provider: "claude", Model: "opus-4-8", CostUSD: 157},
		{Provider: "codex", Model: "gpt-5.4", CostUSD: 54},
		{Provider: "claude", Model: "opus-4-7", CostUSD: 33},
		{Provider: "codex", Model: "gpt-5.3", CostUSD: 2},
	}
	// Default (empty) sort clusters by provider: codex combined ($442) outranks
	// claude ($190), and models are cost-desc within each band.
	SortUsageGroups(groups, "provider,model", "", "")
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.Provider + "/" + g.Model
	}
	require.Equal(t, []string{
		"codex/gpt-5.5", "codex/gpt-5.4", "codex/gpt-5.3",
		"claude/opus-4-8", "claude/opus-4-7",
	}, got)
}

func TestSortUsageGroups_explicitCostStaysFlat(t *testing.T) {
	groups := []UsageGroup{
		{Provider: "codex", Model: "gpt-5.5", CostUSD: 386},
		{Provider: "claude", Model: "opus-4-8", CostUSD: 157},
		{Provider: "codex", Model: "gpt-5.3", CostUSD: 2},
	}
	SortUsageGroups(groups, "provider,model", "cost", "desc")
	require.Equal(t, "codex", groups[0].Provider)
	require.Equal(t, "claude", groups[1].Provider)
	require.Equal(t, "codex", groups[2].Provider)
}

func TestRunHistoryProviders_pricesAndJSON(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	p := &HistoryProviderMock{
		NameFunc: func() string { return "codex" },
		LocalUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return []TokenRecord{{
				Provider:  "codex",
				Model:     "gpt-5-codex",
				SessionID: "s1",
				Timestamp: ts,
				Tokens:    TokenBreakdown{Input: 1_000_000},
			}}, nil
		},
	}
	report := RunHistoryProviders(context.Background(), []HistoryProvider{p}, nil, HistoryOptions{}, defaultPricer(), "provider,model")
	require.Len(t, report.Groups, 1)
	require.InDelta(t, 1.25, report.Total.CostUSD, 1e-9)
	require.Len(t, p.LocalUsageCalls(), 1)

	var buf bytes.Buffer
	require.NoError(t, EncodeHistoryJSON(&buf, report))
	var decoded HistoryReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	require.Equal(t, "provider,model", decoded.GroupBy)
}

func TestRunHistoryProviders_emptyRecordsEncodeAsArray(t *testing.T) {
	p := &HistoryProviderMock{
		NameFunc: func() string { return "codex" },
		LocalUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return nil, nil
		},
	}
	report := RunHistoryProviders(context.Background(), []HistoryProvider{p}, nil, HistoryOptions{}, defaultPricer(), "provider,model")
	require.Empty(t, report.Groups)
	require.NotNil(t, report.Providers[0].Records)

	var buf bytes.Buffer
	require.NoError(t, EncodeHistoryJSON(&buf, report))
	require.Contains(t, buf.String(), `"records": []`)
}

// TestRunHistoryProviders_partialFailure exercises the mixed success/error path:
// one provider returns records, another errors. The successful provider's data
// must still be priced and grouped, the failing one must carry an error + hint,
// and ExitErrorIfAllHistoryProvidersFailed must NOT trip since one succeeded.
func TestRunHistoryProviders_partialFailure(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	ok := &HistoryProviderMock{
		NameFunc: func() string { return "codex" },
		LocalUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return []TokenRecord{{
				Provider:  "codex",
				Model:     "gpt-5-codex",
				SessionID: "s1",
				Timestamp: ts,
				Tokens:    TokenBreakdown{Input: 1_000_000},
			}}, nil
		},
	}
	bad := &HistoryProviderMock{
		NameFunc: func() string { return "claude" },
		LocalUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return nil, errors.New("read ~/.claude: permission denied")
		},
	}

	report := RunHistoryProviders(context.Background(), []HistoryProvider{ok, bad}, nil, HistoryOptions{}, defaultPricer(), "provider,model")

	// Both providers were consulted exactly once.
	require.Len(t, ok.LocalUsageCalls(), 1)
	require.Len(t, bad.LocalUsageCalls(), 1)

	// Results carry per-provider status; order matches input order.
	require.Len(t, report.Providers, 2)
	byName := map[string]HistoryProviderResult{}
	for _, r := range report.Providers {
		byName[r.Name] = r
	}
	require.True(t, byName["codex"].OK)
	require.False(t, byName["claude"].OK)
	require.Contains(t, byName["claude"].Error, "permission denied")
	require.NotEmpty(t, byName["claude"].Hint, "failing provider should get a remediation hint")
	require.Empty(t, byName["codex"].Hint, "successful provider should not get a hint")

	// The successful provider's records are still priced and grouped.
	require.Len(t, report.Groups, 1)
	require.Equal(t, "codex", report.Groups[0].Provider)
	require.InDelta(t, 1.25, report.Total.CostUSD, 1e-9)

	// One success means the all-failed guard does not trip.
	require.NoError(t, ExitErrorIfAllHistoryProvidersFailed(report))
}
