package presentation

import (
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestHumanizeCount(t *testing.T) {
	require.Equal(t, "999", humanizeCount(999))
	require.Equal(t, "1.0K", humanizeCount(1000))
	require.Equal(t, "3.3K", humanizeCount(3343))
	require.Equal(t, "378.1K", humanizeCount(378055))
	require.Equal(t, "1.0M", humanizeCount(1_000_000))
	require.Equal(t, "16.5M", humanizeCount(16_481_153))
	require.Equal(t, "1.0B", humanizeCount(1_000_000_000))
}

func TestFormatCurrency(t *testing.T) {
	require.Equal(t, "$0.9999", formatCurrency(0.99994, false, true))
	require.Equal(t, "$1.00", formatCurrency(1.0, false, true))
	require.Equal(t, "$33.29", formatCurrency(33.286, false, true))
	require.Equal(t, "$0.46", formatCurrency(0.4599, true, false))
	require.Equal(t, "$1.00", formatCurrency(0.99994, false, false))
}

func TestComputeShare(t *testing.T) {
	require.InDelta(t, 74.1, roundHalfUp(computeShare(74.1, 100), 1), 1e-9)
	require.Zero(t, computeShare(1, 0))
}

func TestHistoryRows_formatsAndOmitsColumns(t *testing.T) {
	report := ai.HistoryReport{
		GroupBy: "provider,model",
		Groups: []ai.UsageGroup{
			{
				Provider: "codex",
				Model:    "gpt-5.5",
				Tokens:   ai.TokenBreakdown{Input: 16_481_153, CacheRead: 378_055, Output: 3_343},
				CostUSD:  437.0053,
				Records:  1,
			},
			{
				Provider: "claude",
				Model:    "claude-sonnet-4",
				Tokens:   ai.TokenBreakdown{Input: 999, Output: 100},
				CostUSD:  0.013,
				Records:  1,
			},
		},
		Total: ai.UsageGroup{
			Tokens:  ai.TokenBreakdown{Input: 16_482_152, CacheRead: 378_055, Output: 3_443},
			CostUSD: 437.0183,
			Records: 2,
		},
	}
	headers, rows := historyRows(report, HistoryPrintOptions{})
	require.NotContains(t, headers, "cache write")
	require.NotContains(t, headers, "reasoning")
	require.NotContains(t, headers, "total")
	require.Contains(t, rows[0], "16.5M")
	require.Contains(t, trimmedCells(rows[0]), "$437.01")
	require.Contains(t, trimmedCells(rows[1]), "$0.01")
	require.Contains(t, trimmedCells(rows[len(rows)-1]), "100.0%")
	require.Contains(t, trimmedCells(rows[len(rows)-1]), "$437.02")
	require.Equal(t, int64(16_482_152), report.Total.Tokens.Input)
}

func TestHistoryRows_usesAdaptiveCurrencyForSingleSummaryRow(t *testing.T) {
	report := ai.HistoryReport{
		GroupBy: "provider,model",
		Groups: []ai.UsageGroup{{
			Provider: "codex",
			Model:    "gpt-5.5",
			Tokens:   ai.TokenBreakdown{Input: 1000},
			CostUSD:  0.0958,
		}},
		Total: ai.UsageGroup{Tokens: ai.TokenBreakdown{Input: 1000}, CostUSD: 0.0958},
	}
	_, rows := historyRows(report, HistoryPrintOptions{})
	require.Contains(t, trimmedCells(rows[0]), "$0.0958")
	require.Contains(t, trimmedCells(rows[1]), "$0.10")
}

func TestHistoryRows_usesFlatCurrencyForDateTables(t *testing.T) {
	report := ai.HistoryReport{
		GroupBy: "date",
		Groups: []ai.UsageGroup{{
			Date:    "2026-06-03",
			Tokens:  ai.TokenBreakdown{Input: 1000},
			CostUSD: 0.0958,
		}},
		Total: ai.UsageGroup{Tokens: ai.TokenBreakdown{Input: 1000}, CostUSD: 0.0958},
	}
	_, rows := historyRows(report, HistoryPrintOptions{})
	require.Contains(t, trimmedCells(rows[0]), "$0.10")
}

func TestHistoryRows_sessionModelSplit(t *testing.T) {
	report := ai.HistoryReport{
		GroupBy: "session,model",
		Groups: []ai.UsageGroup{{
			Provider:  "codex",
			SessionID: "167c5823-ebfa-4266-88c9-945ec5cf7b65",
			Model:     "gpt-5.5",
			Tokens:    ai.TokenBreakdown{Input: 1000},
			CostUSD:   1,
		}},
		Total: ai.UsageGroup{Tokens: ai.TokenBreakdown{Input: 1000}, CostUSD: 1},
	}
	headers, rows := historyRows(report, HistoryPrintOptions{})
	require.Equal(t, []string{"provider", "session", "model", "input", "cache read", "output", "share", "api equiv"}, headers)
	require.Equal(t, "167c5823", rows[0][1])
}

func TestShortSessionID_prefersEmbeddedUUIDSegment(t *testing.T) {
	require.Equal(t, "019e8f5a", shortSessionID("rollout-2026-06-03T14-18-43-019e8f5a-5749-7880-b60d-937564e52714"))
}

func TestHistoryRows_reasoningDashWhenMixed(t *testing.T) {
	report := ai.HistoryReport{
		GroupBy: "provider,model",
		Groups: []ai.UsageGroup{
			{Provider: "claude", Model: "claude-sonnet-4", Tokens: ai.TokenBreakdown{Input: 1}, CostUSD: 1},
			{Provider: "codex", Model: "gpt-5.5", Tokens: ai.TokenBreakdown{Input: 1, Reasoning: 10}, CostUSD: 1},
		},
		Total: ai.UsageGroup{Tokens: ai.TokenBreakdown{Input: 2, Reasoning: 10}, CostUSD: 2},
	}
	headers, rows := historyRows(report, HistoryPrintOptions{})
	require.Contains(t, headers, "reasoning")
	reasoningCol := indexOf(headers, "reasoning")
	require.Equal(t, dash, strings.TrimSpace(rows[0][reasoningCol]))
}

func TestAlignedTableLines_trimsTrailingWhitespace(t *testing.T) {
	lines := alignedTableLines(
		[]string{"group", "api equiv"},
		[][]string{
			{"codex", "$3.47  "},
			{"total", "$10.79"},
		},
	)
	for _, line := range lines {
		require.False(t, strings.HasSuffix(line, " "), line)
	}
}

func TestSortUsageGroups(t *testing.T) {
	groups := []ai.UsageGroup{
		{Model: "cheap", CostUSD: 1, Tokens: ai.TokenBreakdown{Input: 100}, Date: "2026-06-02"},
		{Model: "expensive", CostUSD: 10, Tokens: ai.TokenBreakdown{Input: 10}, Date: "2026-06-01"},
	}
	ai.SortUsageGroups(groups, "model", "cost", "desc")
	require.Equal(t, "expensive", groups[0].Model)
	ai.SortUsageGroups(groups, "date", "date", "asc")
	require.Equal(t, "expensive", groups[0].Model)
}

func indexOf(values []string, want string) int {
	for i, v := range values {
		if v == want {
			return i
		}
	}
	return -1
}

func trimmedCells(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.TrimSpace(v)
	}
	return out
}
