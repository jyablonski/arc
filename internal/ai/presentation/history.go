package presentation

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

type HistoryPrintOptions struct {
	ShowTotalTokens bool
	Now             time.Time
}

// PrintHistory renders the token history as one grid with a total row, and
// closes with what the listed usage would cost at API rates.
func PrintHistory(w io.Writer, report ai.HistoryReport, opts HistoryPrintOptions) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	style := output.StyleFor(w)
	meta := output.Timestamp(report.FetchedAt)
	if report.FetchedAt.IsZero() {
		meta = output.Timestamp(opts.Now)
	}
	sc := &output.Screen{W: w, Style: style, Title: "arc ai tokens", Meta: meta}

	if len(report.Groups) > 0 {
		headers, rows := historyRows(report, opts)
		grid := output.Grid{Columns: make([]output.Column, len(headers)), Rows: rows}
		for i, h := range headers {
			grid.Columns[i] = output.Column{Header: h}
			switch h {
			case "group", "provider", "session", "model":
			default:
				grid.Columns[i].Align = output.AlignRight
			}
		}
		for _, row := range rows {
			for i, cell := range row {
				if strings.TrimSpace(cell) == dash {
					row[i] = strings.Replace(cell, dash, style.Dash(), 1)
				}
			}
		}
		sc.Grid(grid)
	}

	var notes []output.Note
	for _, p := range report.Providers {
		if !p.OK {
			notes = append(notes, output.Note{Glyph: output.GlyphFail, Label: p.Name, Detail: p.Error, Hint: p.Hint})
		}
	}
	if len(notes) > 0 {
		if len(report.Groups) > 0 {
			sc.Blank()
		}
		sc.Notes(notes)
	}

	sc.Flush(historyFooter(style, report))
}

func historyFooter(style output.Style, report ai.HistoryReport) string {
	if len(report.Groups) == 0 {
		return style.Glyph(output.GlyphInfo) + " no local token usage records found"
	}
	parts := []string{
		output.Count(len(report.Groups), "group", "groups"),
		humanizeCount(report.Total.Tokens.Total()) + " tokens",
		formatCurrency(report.Total.CostUSD, true, false) + " api equiv",
	}
	return style.Faint(strings.Join(parts, style.Sep()))
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
	alignDecimalColumns(rows, headers)
	return headers, rows
}

func shouldUseAdaptiveCurrency(report ai.HistoryReport) bool {
	return len(report.Groups) <= 1 && report.GroupBy != "date"
}

func groupCells(g ai.UsageGroup, groupBy string) []string {
	switch groupBy {
	case "provider":
		return []string{g.Provider}
	case "model":
		return []string{g.Model}
	case "date":
		return []string{g.Date}
	case "session,model":
		return []string{g.Provider, shortSessionID(g.SessionID), g.Model}
	default:
		return []string{g.Provider + "/" + g.Model}
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

// alignDecimalColumns lines up decimal points in columns that can mix
// precisions (adaptive currency); the grid right-aligns everything else.
func alignDecimalColumns(rows [][]string, headers []string) {
	for col, header := range headers {
		if header != "share" && header != "api equiv" {
			continue
		}
		values := make([]string, len(rows))
		for i := range rows {
			values[i] = rows[i][col]
		}
		aligned := alignDecimal(values)
		width := 0
		for _, v := range aligned {
			width = max(width, len(v))
		}
		for i := range rows {
			rows[i][col] = fmt.Sprintf("%-*s", width, aligned[i])
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

// roiLines renders the subscription-value block embedded in `arc ai usage`.
func roiLines(style output.Style, summary ROISummary) []string {
	if len(summary.Entries) == 0 {
		return nil
	}
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
	grid := output.Grid{
		Columns: []output.Column{
			{Header: "provider"},
			{Header: "api equiv", Align: output.AlignRight},
			{Header: "subscription", Align: output.AlignRight},
			{Header: "multiple", Align: output.AlignRight},
		},
		Rows: rows,
	}
	lines := []string{style.Faint("subscription ROI  " + summary.WindowLabel)}
	return append(lines, grid.Lines(style, style.Width)...)
}

func formatMultiple(v float64) string {
	return fmt.Sprintf("%.1fx", roundHalfUp(v, 1))
}
