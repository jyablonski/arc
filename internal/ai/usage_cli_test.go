package ai

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseProviderCSV(t *testing.T) {
	require.Nil(t, ParseProviderCSV(""))
	require.Nil(t, ParseProviderCSV("   "))
	require.Equal(t, []string{"claude"}, ParseProviderCSV("claude"))
	require.Equal(t, []string{"claude", "codex"}, ParseProviderCSV(" Claude , Codex "))
	require.Equal(t, []string{"cursor"}, ParseProviderCSV(",cursor,"))
}

func TestValidateProviderFilters(t *testing.T) {
	require.NoError(t, ValidateProviderFilters(nil))
	require.NoError(t, ValidateProviderFilters([]string{}))
	require.NoError(t, ValidateProviderFilters([]string{"claude", "cursor"}))
	err := ValidateProviderFilters([]string{"claude", "wat"})
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown provider "wat"`)
}

func TestEncodeAggregateJSON_roundTrip(t *testing.T) {
	ts := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	agg := AggregateReport{
		FetchedAt: ts,
		Providers: []ProviderResult{
			{Name: "claude", OK: true, Report: UsageReport{Windows: []UsageWindow{{Label: "5 hour", PercentUsed: 10}}}},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, EncodeAggregateJSON(&buf, agg))
	require.Contains(t, buf.String(), `"name": "claude"`)
	require.Contains(t, buf.String(), `"fetched_at"`)

	var decoded AggregateReport
	require.NoError(t, json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&decoded))
	require.Len(t, decoded.Providers, 1)
	require.True(t, decoded.Providers[0].OK)
}

func TestExitErrorIfAllProvidersFailed(t *testing.T) {
	err := ExitErrorIfAllProvidersFailed(AggregateReport{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no providers selected")

	err = ExitErrorIfAllProvidersFailed(AggregateReport{Providers: []ProviderResult{
		{Name: "a", OK: false, Error: "e1"},
		{Name: "b", OK: false, Error: "e2"},
	}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "e1")

	require.NoError(t, ExitErrorIfAllProvidersFailed(AggregateReport{Providers: []ProviderResult{
		{Name: "a", OK: false, Error: "bad"},
		{Name: "b", OK: true, Report: UsageReport{}},
	}}))

	require.NoError(t, ExitErrorIfAllProvidersFailed(AggregateReport{Providers: []ProviderResult{
		{Name: "a", OK: true, Report: UsageReport{}},
	}}))
}

func TestCombineErrors_exitErrorConsistency(t *testing.T) {
	err := ExitErrorIfAllProvidersFailed(AggregateReport{Providers: []ProviderResult{
		{Name: "x", OK: false, Error: "boom"},
	}})
	require.Error(t, err)
	require.Equal(t, CombineErrors(AggregateReport{Providers: []ProviderResult{
		{Name: "x", OK: false, Error: "boom"},
	}}), err)
}
