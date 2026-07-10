package output

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
)

// ansiPattern matches SGR escape sequences (e.g. color codes) so table layout
// can measure and pad cells by their visible width rather than byte length.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleWidth returns the display width of s with ANSI escape sequences removed.
func visibleWidth(s string) int {
	return len([]rune(ansiPattern.ReplaceAllString(s, "")))
}

var (
	headerColor  = color.New(color.FgCyan, color.Bold)
	successColor = color.New(color.FgGreen, color.Bold)
	errorColor   = color.New(color.FgRed, color.Bold)
	infoColor    = color.New(color.FgBlue)
	warningColor = color.New(color.FgYellow)
)

func Header(s string) {
	fmt.Println()
	_, _ = headerColor.Println(s)
	fmt.Println(strings.Repeat("-", len(s)))
}

// SectionAccent prints a titled block with underline in the given ANSI style (stdout).
func SectionAccent(title string, accent *color.Color) {
	fmt.Println()
	_, _ = accent.Fprintf(os.Stdout, "%s\n", title)
	_, _ = accent.Fprintf(os.Stdout, "%s\n", strings.Repeat("─", len(title)))
}

func Success(s string) {
	_, _ = successColor.Printf("✓ %s\n", s)
}

func Error(s string) {
	_, _ = errorColor.Printf("✗ %s\n", s)
}

func Info(s string) {
	_, _ = infoColor.Printf("i %s\n", s)
}

func Warning(s string) {
	_, _ = warningColor.Printf("⚠ %s\n", s)
}

func Print(s string) {
	fmt.Println(s)
}

// Table prints a left-aligned table to stdout in arc's canonical
// structured-text format: lowercase headers, a per-column underline rule, and a
// two-space gutter between columns (matches `arc ai tokens`). It is the single
// supported way to render tabular output across the CLI — prefer it over
// hand-rolled tabwriter or Printf alignment so every command reads the same.
func Table(headers []string, rows [][]string) {
	FprintTable(os.Stdout, headers, rows)
}

// FprintTable writes a canonical table (see Table) to an arbitrary writer.
func FprintTable(w io.Writer, headers []string, rows [][]string) {
	for _, line := range TableLines(headers, rows) {
		_, _ = fmt.Fprintln(w, line)
	}
}

// TableLines renders a canonical table (see Table) as individual lines without
// printing them, for callers that need to embed or test the output. Headers are
// lowercased and each line is right-trimmed of padding.
func TableLines(headers []string, rows [][]string) []string {
	cols := make([]string, len(headers))
	widths := make([]int, len(headers))
	for i, h := range headers {
		cols[i] = strings.ToLower(h)
		widths[i] = visibleWidth(cols[i])
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := visibleWidth(cell); i < len(widths) && w > widths[i] {
				widths[i] = w
			}
		}
	}

	lines := make([]string, 0, len(rows)+2)
	appendRow := func(cells []string) {
		var b strings.Builder
		for i, cell := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(cell)
			if pad := widths[i] - visibleWidth(cell); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}

	appendRow(cols)
	rule := make([]string, len(widths))
	for i, w := range widths {
		rule[i] = strings.Repeat("-", w)
	}
	appendRow(rule)
	for _, row := range rows {
		appendRow(row)
	}
	return lines
}
