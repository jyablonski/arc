package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticPricer_Cost(t *testing.T) {
	cost, source := NewStaticPricer().Cost("gpt-5-codex", TokenBreakdown{
		Input:     1_000_000,
		CacheRead: 1_000_000,
		Output:    1_000_000,
		Reasoning: 1_000_000,
	})
	require.Equal(t, "static-openai-api", source)
	require.InDelta(t, 21.375, cost, 1e-9)

	cost, source = NewStaticPricer().Cost("gpt-5.5", TokenBreakdown{Input: 1_000_000})
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

func TestRunHistoryProviders_pricesAndJSON(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	p := fakeHistoryProvider{
		nameFunc: func() string { return "codex" },
		localUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return []TokenRecord{{
				Provider:  "codex",
				Model:     "gpt-5-codex",
				SessionID: "s1",
				Timestamp: ts,
				Tokens:    TokenBreakdown{Input: 1_000_000},
			}}, nil
		},
	}
	report := RunHistoryProviders(context.Background(), []HistoryProvider{p}, nil, HistoryOptions{}, NewStaticPricer(), "provider,model")
	require.Len(t, report.Groups, 1)
	require.InDelta(t, 1.25, report.Total.CostUSD, 1e-9)

	var buf bytes.Buffer
	require.NoError(t, EncodeHistoryJSON(&buf, report))
	var decoded HistoryReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	require.Equal(t, "provider,model", decoded.GroupBy)
}

func TestRunHistoryProviders_emptyRecordsEncodeAsArray(t *testing.T) {
	p := fakeHistoryProvider{
		nameFunc: func() string { return "codex" },
		localUsageFunc: func(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
			return nil, nil
		},
	}
	report := RunHistoryProviders(context.Background(), []HistoryProvider{p}, nil, HistoryOptions{}, NewStaticPricer(), "provider,model")
	require.Empty(t, report.Groups)
	require.NotNil(t, report.Providers[0].Records)

	var buf bytes.Buffer
	require.NoError(t, EncodeHistoryJSON(&buf, report))
	require.Contains(t, buf.String(), `"records": []`)
}

type fakeHistoryProvider struct {
	nameFunc       func() string
	localUsageFunc func(context.Context, HistoryOptions) ([]TokenRecord, error)
}

func (f fakeHistoryProvider) Name() string {
	return f.nameFunc()
}

func (f fakeHistoryProvider) LocalUsage(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error) {
	return f.localUsageFunc(ctx, opts)
}
