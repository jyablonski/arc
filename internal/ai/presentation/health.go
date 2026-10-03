package presentation

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

// healthSectionRank orders health rows by provider, matching the provider order
// used by "arc ai usage"/"ai tokens"; machine-wide checks ("local") come last.
var healthSectionRank = map[string]int{"claude": 0, "codex": 1, "cursor": 2, "local": 3}

// PrintHealth renders every check in one grid ordered by provider, lists the
// fix for each non-OK check beneath it, and closes with a verdict line.
func PrintHealth(w io.Writer, report ai.HealthReport) {
	style := output.StyleFor(w)
	meta := output.Timestamp(report.FetchedAt)
	if report.FetchedAt.IsZero() {
		meta = output.Timestamp(time.Now())
	}
	sc := &output.Screen{W: w, Style: style, Title: "arc ai health", Meta: meta}
	if len(report.Checks) == 0 {
		sc.Flush(style.Glyph(output.GlyphInfo) + " no health checks ran")
		return
	}

	checks := append([]ai.HealthCheck(nil), report.Checks...)
	sort.SliceStable(checks, func(i, j int) bool {
		return healthRank(checks[i]) < healthRank(checks[j])
	})

	grid := output.Grid{Columns: []output.Column{
		{Align: output.AlignCenter},
		{Header: "provider"},
		{Header: "check"},
		{Header: "detail", Flex: true},
	}}
	var notes []output.Note
	warned, failed := 0, 0
	for _, c := range checks {
		section, label := healthSection(c)
		provider := strings.ToLower(section)
		grid.Rows = append(grid.Rows, []string{style.Glyph(healthGlyph(c.Status)), provider, label, c.Detail})
		switch c.Status {
		case ai.HealthWarn:
			warned++
		case ai.HealthFail:
			failed++
		}
		// The common all-green case lists no hints.
		if c.Status != ai.HealthOK && c.Hint != "" {
			notes = append(notes, output.Note{Glyph: healthGlyph(c.Status), Label: provider + "/" + label, Detail: c.Hint})
		}
	}
	sc.Grid(grid)
	if len(notes) > 0 {
		sc.Blank()
		sc.Notes(notes)
	}
	sc.Flush(healthVerdict(style, len(checks), warned, failed))
}

func healthVerdict(style output.Style, total, warned, failed int) string {
	glyph := output.GlyphOK
	var parts []string
	if failed > 0 {
		glyph = output.GlyphFail
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if warned > 0 {
		if failed == 0 {
			glyph = output.GlyphWarn
		}
		parts = append(parts, output.Count(warned, "warning", "warnings"))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("all %s passed", output.Count(total, "check", "checks")))
	} else {
		parts = append(parts, output.Count(total, "check", "checks"))
	}
	return style.Glyph(glyph) + " " + strings.Join(parts, style.Sep())
}

// healthSection maps a check to its display section and row label: provider
// auth/tooling checks group under the provider; everything else is machine-wide.
func healthSection(c ai.HealthCheck) (section, label string) {
	switch c.Category {
	case "auth", "tooling":
		return c.Name, c.Category
	default:
		return "local", c.Name
	}
}

func healthRank(c ai.HealthCheck) int {
	section, _ := healthSection(c)
	if r, ok := healthSectionRank[strings.ToLower(section)]; ok {
		return r
	}
	return len(healthSectionRank)
}

func healthGlyph(s ai.HealthStatus) output.Glyph {
	switch s {
	case ai.HealthOK:
		return output.GlyphOK
	case ai.HealthFail:
		return output.GlyphFail
	default:
		return output.GlyphWarn
	}
}
