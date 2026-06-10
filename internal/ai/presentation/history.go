package presentation

import (
	"fmt"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

type HistoryPrintOptions struct {
	ShowTotalTokens bool
}

func PrintHistory(report ai.HistoryReport, opts HistoryPrintOptions) {
	output.SectionAccent("AI token history", providerAccent("codex"))
	if len(report.Groups) == 0 {
		output.Info("no local token usage records found")
		return
	}

	headers, rows := historyRows(report, opts)
	output.Table(headers, rows)
}

func historyRows(report ai.HistoryReport, opts HistoryPrintOptions) ([]string, [][]string) {
	showCacheWrite := report.Total.Tokens.CacheWrite != 0
	showReasoning := report.Total.Tokens.Reasoning != 0
	showSessionModel := report.GroupBy == "session,model"
	adaptiveCurrency := shouldUseAdaptiveCurrency(report)

	headers := []string{}
	if showSessionModel {
		headers = append(headers, "provider", "session", "model")
	} else {
		headers = append(headers, "group")
	}
	headers = append(headers, "input", "cache read")
	if showCacheWrite {
		headers = append(headers, "cache write")
	}
	headers = append(headers, "output")
	if showReasoning {
		headers = append(headers, "reasoning")
	}
	if opts.ShowTotalTokens {
		headers = append(headers, "total")
	}
	headers = append(headers, "share", "api equiv")

	rows := make([][]string, 0, len(report.Groups)+1)
	for _, g := range report.Groups {
		row := groupCells(g, report.GroupBy)
		row = append(row, tokenCells(g.Tokens, showCacheWrite, showReasoning, opts.ShowTotalTokens)...)
		row = append(row, formatShare(computeShare(g.CostUSD, report.Total.CostUSD)), formatCurrency(g.CostUSD, false, adaptiveCurrency))
		rows = append(rows, row)
	}
	row := totalGroupCells(showSessionModel)
	row = append(row, tokenCells(report.Total.Tokens, showCacheWrite, showReasoning, opts.ShowTotalTokens)...)
	row = append(row, "100.0%", formatCurrency(report.Total.CostUSD, true, false))
	rows = append(rows, row)
	alignNumericColumns(rows, headers)
	return headers, rows
}

func shouldUseAdaptiveCurrency(report ai.HistoryReport) bool {
	return len(report.Groups) <= 1 && report.GroupBy != "date"
}

func groupCells(g ai.UsageGroup, groupBy string) []string {
	accent := providerAccent(g.Provider)
	switch groupBy {
	case "provider":
		return []string{accent.Sprint(g.Provider)}
	case "model":
		return []string{accent.Sprint(g.Model)}
	case "date":
		return []string{g.Date}
	case "session,model":
		return []string{accent.Sprint(g.Provider), shortSessionID(g.SessionID), accent.Sprint(g.Model)}
	default:
		return []string{accent.Sprint(g.Provider + "/" + g.Model)}
	}
}

func totalGroupCells(sessionModel bool) []string {
	if sessionModel {
		return []string{"total", "", ""}
	}
	return []string{"total"}
}

func tokenCells(t ai.TokenBreakdown, showCacheWrite, showReasoning, showTotal bool) []string {
	cells := []string{
		humanizeCount(t.Input),
		humanizeCount(t.CacheRead),
	}
	if showCacheWrite {
		cells = append(cells, zeroDash(t.CacheWrite))
	}
	cells = append(cells, humanizeCount(t.Output))
	if showReasoning {
		cells = append(cells, zeroDash(t.Reasoning))
	}
	if showTotal {
		cells = append(cells, humanizeCount(t.Total()))
	}
	return cells
}

func zeroDash(v int64) string {
	if v == 0 {
		return dash
	}
	return humanizeCount(v)
}

func alignNumericColumns(rows [][]string, headers []string) {
	for col, header := range headers {
		switch header {
		case "share", "api equiv":
			values := make([]string, len(rows))
			for i := range rows {
				values[i] = rows[i][col]
			}
			aligned := alignDecimal(values)
			for i := range rows {
				rows[i][col] = aligned[i]
			}
		case "input", "cache read", "cache write", "output", "reasoning", "total":
			width := 0
			for _, row := range rows {
				if len(row[col]) > width {
					width = len(row[col])
				}
			}
			for i := range rows {
				rows[i][col] = fmt.Sprintf("%*s", width, rows[i][col])
			}
		}
	}
}

type ROIEntry struct {
	Provider        string
	EquivalentCost  float64
	SubscriptionUSD float64
	Multiple        float64
}

type ROISummary struct {
	WindowLabel      string
	Entries          []ROIEntry
	EquivalentCost   float64
	SubscriptionCost float64
	Multiple         float64
}

func PrintROISummary(summary ROISummary) {
	if len(summary.Entries) == 0 {
		return
	}
	output.SectionAccent("Subscription ROI", providerAccent("claude"))
	fmt.Printf("window: %s\n", summary.WindowLabel)
	rows := make([][]string, 0, len(summary.Entries)+1)
	for _, e := range summary.Entries {
		rows = append(rows, []string{
			e.Provider,
			formatCurrency(e.EquivalentCost, true, false),
			formatCurrency(e.SubscriptionUSD, true, false),
			formatMultiple(e.Multiple),
		})
	}
	rows = append(rows, []string{
		"total",
		formatCurrency(summary.EquivalentCost, true, false),
		formatCurrency(summary.SubscriptionCost, true, false),
		formatMultiple(summary.Multiple),
	})
	headers := []string{"provider", "api equiv", "subscription", "multiple"}
	alignNumericColumns(rows, headers)
	output.Table(headers, rows)
}

func formatMultiple(v float64) string {
	return fmt.Sprintf("%.1fx", roundHalfUp(v, 1))
}
