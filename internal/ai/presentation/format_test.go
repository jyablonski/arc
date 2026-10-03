package presentation

import (
	"bytes"
	"strings"
	"testing"
	"time"

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

func TestPrintHistory_screenWithTotalRowAndFooter(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	now := time.Date(2026, 10, 3, 9, 55, 26, 0, time.UTC)
	report := ai.HistoryReport{
		FetchedAt: now,
		GroupBy:   "provider,model",
		Providers: []ai.HistoryProviderResult{{Name: "claude", OK: true}, {Name: "codex", Error: "no logs", Hint: "install codex"}},
		Groups: []ai.UsageGroup{
			{Provider: "claude", Model: "claude-opus-5-5", Tokens: ai.TokenBreakdown{Input: 934, CacheRead: 116_600_000, CacheWrite: 1_200_000, Output: 447_200}, CostUSD: 38.18},
			{Provider: "claude", Model: "claude-haiku-4-5", Tokens: ai.TokenBreakdown{Input: 1_300_000, CacheRead: 43_600_000, Output: 316_300}, CostUSD: 0.84},
		},
		Total: ai.UsageGroup{Tokens: ai.TokenBreakdown{Input: 1_300_934, CacheRead: 160_200_000, CacheWrite: 1_200_000, Output: 763_500}, CostUSD: 39.02},
	}
	var buf bytes.Buffer
	PrintHistory(&buf, report, HistoryPrintOptions{Now: now})
	lines := strings.Split(buf.String(), "\n")

	require.True(t, strings.HasPrefix(lines[0], "arc ai tokens"))
	require.True(t, strings.HasSuffix(lines[0], "2026-10-03 09:55:26"))
	// Numeric headers sit over right-aligned cells; empty cells are marked.
	require.Equal(t, "group                    input  cache read  cache write  output   share  api equiv", lines[3])
	require.Equal(t, "claude/claude-opus-5-5     934      116.6M         1.2M  447.2K   97.8%     $38.18", lines[4])
	require.Equal(t, "claude/claude-haiku-4-5   1.3M       43.6M            —  316.3K    2.2%      $0.84", lines[5])
	require.Equal(t, "total                     1.3M      160.2M         1.2M  763.5K  100.0%     $39.02", lines[6])
	require.Contains(t, buf.String(), "✗ codex  no logs  install codex")
	require.Contains(t, buf.String(), "2 groups · 163.5M tokens · $39.02 api equiv")
}

func TestPrintHistory_empty(t *testing.T) {
	var buf bytes.Buffer
	PrintHistory(&buf, ai.HistoryReport{}, HistoryPrintOptions{})
	require.Contains(t, buf.String(), "arc ai tokens")
	require.Contains(t, buf.String(), "no local token usage records found")
}
