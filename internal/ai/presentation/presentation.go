package presentation

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

const (
	usageBarWidth = 16
	// lowRemainPct is where a window stops being "clear" and the verdict
	// starts asking for attention.
	lowRemainPct = 20.0
)

// UsageOptions controls how `arc ai usage` is rendered.
type UsageOptions struct {
	Now time.Time
	// Cached marks a report served from the local cache.
	Cached bool
	// Short prints only the verdict line.
	Short bool
	// ROI is the optional subscription-value block shown under the table.
	ROI ROISummary
}

type usageRow struct {
	provider string
	window   string
	used     float64 // < 0 when the provider reported no percentage
	resetsAt *time.Time
}

// PrintUsage renders the aggregate quota report as one table across
// providers, led by what is consumed, and closes with a verdict line.
func PrintUsage(w io.Writer, agg ai.AggregateReport, opts UsageOptions) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	style := output.StyleFor(w)
	rows, failed := usageRows(agg)
	verdict := usageVerdict(style, rows, failed, opts.Short)
	if opts.Short {
		_, _ = fmt.Fprintln(w, verdict)
		return
	}

	meta := output.Timestamp(agg.FetchedAt)
	if agg.FetchedAt.IsZero() {
		meta = output.Timestamp(opts.Now)
	}
	if opts.Cached {
		meta += style.Sep() + "cached"
	}
	sc := &output.Screen{W: w, Style: style, Title: "arc ai usage", Meta: meta}

	if len(rows) > 0 {
		grid := output.Grid{Columns: []output.Column{
			{Header: "provider"},
			{Header: "window"},
			{Header: "used"},
			{Header: "left", Align: output.AlignRight},
			{Header: "resets"},
		}}
		for _, r := range rows {
			grid.Rows = append(grid.Rows, []string{
				r.provider,
				strings.ReplaceAll(r.window, " · ", style.Sep()),
				usageBar(style, r.used),
				formatLeft(style, r.used),
				formatResets(style, r, opts.Now),
			})
		}
		sc.Grid(grid)
	}

	var notes []output.Note
	for _, pr := range agg.Providers {
		switch {
		case pr.OK && len(pr.Report.Windows) > 0:
		case pr.OK:
			notes = append(notes, output.Note{Glyph: output.GlyphInfo, Label: pr.Name, Detail: "no usage windows returned"})
		default:
			notes = append(notes, output.Note{Glyph: output.GlyphFail, Label: pr.Name, Detail: pr.Error, Hint: pr.Hint})
		}
	}
	if len(notes) > 0 {
		if len(rows) > 0 {
			sc.Blank()
		}
		sc.Notes(notes)
	}

	if roi := roiLines(style, opts.ROI); len(roi) > 0 {
		sc.Blank()
		for _, l := range roi {
			sc.Line(l)
		}
	}
	sc.Flush(verdict)
}

func usageRows(agg ai.AggregateReport) (rows []usageRow, failed []string) {
	for _, pr := range agg.Providers {
		if !pr.OK {
			failed = append(failed, pr.Name)
			continue
		}
		for _, w := range pr.Report.Windows {
			name := w.Window
			if name == "" {
				name = ai.NormalizeWindow(pr.Name, w.Label)
			}
			rows = append(rows, usageRow{provider: pr.Name, window: name, used: w.PercentUsed, resetsAt: w.ResetsAt})
		}
	}
	return rows, failed
}

// usageVerdict is the one line a returning user reads: whether anything is
// low and which window is tightest.
func usageVerdict(style output.Style, rows []usageRow, failed []string, short bool) string {
	var tight *usageRow
	measured, low, exhausted := 0, 0, 0
	for i := range rows {
		r := &rows[i]
		if r.used < 0 {
			continue
		}
		measured++
		left := remaining(r.used)
		switch {
		case left <= 0:
			exhausted++
		case left < lowRemainPct:
			low++
		}
		if tight == nil || left < remaining(tight.used) {
			tight = r
		}
	}

	sep := style.Sep()
	var glyph output.Glyph
	var parts []string
	switch {
	case measured == 0 && len(failed) > 0:
		glyph = output.GlyphFail
		parts = append(parts, "no usage available")
	case measured == 0:
		glyph = output.GlyphInfo
		parts = append(parts, "no usage windows returned")
	case exhausted > 0:
		glyph = output.GlyphFail
		parts = append(parts, fmt.Sprintf("%s exhausted", output.Count(exhausted, "window", "windows")))
	case low > 0:
		glyph = output.GlyphWarn
		parts = append(parts, fmt.Sprintf("%s low", output.Count(low, "window", "windows")))
	case short:
		glyph = output.GlyphOK
		// Denominator counts every window shown, including any the provider
		// reported without a percentage.
		parts = append(parts, fmt.Sprintf("%d/%d windows clear", measured, len(rows)))
	default:
		glyph = output.GlyphOK
		parts = append(parts, "all windows clear")
	}
	if tight != nil {
		name := tight.provider + " " + tight.window
		if short {
			parts = append(parts, fmt.Sprintf("tightest %s %s", name, leftPercent(tight.used)))
		} else {
			parts = append(parts, fmt.Sprintf("tightest is %s at %s left", name, leftPercent(tight.used)))
		}
	}
	if len(failed) > 0 && measured > 0 {
		if glyph == output.GlyphOK {
			glyph = output.GlyphWarn
		}
		parts = append(parts, strings.Join(failed, ", ")+" unavailable")
	}
	return style.Glyph(glyph) + " " + strings.Join(parts, sep)
}

func remaining(used float64) float64 {
	return math.Min(100, math.Max(0, 100-used))
}

// leftPercent prints whole numbers, keeping one decimal only below 10% where
// the difference is actionable. It floors so any consumption reads below 100%.
func leftPercent(used float64) string {
	left := remaining(used)
	if left < 10 {
		return fmt.Sprintf("%.1f%%", math.Floor(left*10)/10)
	}
	return fmt.Sprintf("%d%%", int(math.Floor(left)))
}

func levelColor(style output.Style, used float64, v string) string {
	left := remaining(used)
	switch {
	case left <= 5:
		return style.Red(v)
	case left < lowRemainPct:
		return style.Yellow(v)
	default:
		return style.Green(v)
	}
}

func formatLeft(style output.Style, used float64) string {
	if used < 0 {
		return style.Dash()
	}
	return levelColor(style, used, leftPercent(used))
}

// usageBar fills with what is consumed. Any consumption shows at least one
// cell so a few percent never looks identical to untouched.
func usageBar(style output.Style, used float64) string {
	full, empty := style.Sym("▓", "#"), style.Sym("░", ".")
	if used < 0 {
		return style.Faint(strings.Repeat(empty, usageBarWidth))
	}
	n := int(math.Round(math.Min(100, used) / 100 * usageBarWidth))
	if used > 0 && n == 0 {
		n = 1
	}
	n = min(max(n, 0), usageBarWidth)
	return levelColor(style, used, strings.Repeat(full, n)) + style.Faint(strings.Repeat(empty, usageBarWidth-n))
}

// formatResets distinguishes a window that has not started (nothing used, no
// reset scheduled) from a provider that simply did not report a reset.
func formatResets(style output.Style, r usageRow, now time.Time) string {
	if r.resetsAt == nil {
		if r.used == 0 {
			return "not started"
		}
		return style.Dash()
	}
	d := r.resetsAt.Sub(now)
	if d <= 0 {
		return "now"
	}
	return "in " + output.Age(d)
}
