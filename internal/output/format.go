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
	fmt.Println(strings.Repeat("─", len([]rune(s))))
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

// Bytes formats a byte count using binary units.
func Bytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case n >= gib:
		return fmt.Sprintf("%.1f GiB", float64(n)/gib)
	case n >= mib:
		return fmt.Sprintf("%.1f MiB", float64(n)/mib)
	case n >= kib:
		return fmt.Sprintf("%.1f KiB", float64(n)/kib)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func Print(s string) {
	fmt.Println(s)
}

// Table prints a left-aligned table to stdout in arc's canonical grid format
// (see Grid): lowercase headers, a two-space gutter, and no underline rule.
// Commands that need alignment control, glyph columns, or a flex column should
// build a Grid directly.
func Table(headers []string, rows [][]string) {
	FprintTable(os.Stdout, headers, rows)
}

// FprintTable writes a canonical table (see Table) to an arbitrary writer.
func FprintTable(w io.Writer, headers []string, rows [][]string) {
	for _, line := range tableGrid(headers, rows).Lines(StyleFor(w), 0) {
		_, _ = fmt.Fprintln(w, line)
	}
}

// TableLines renders a canonical table (see Table) as plain lines without
// printing them, for callers that need to embed or test the output.
func TableLines(headers []string, rows [][]string) []string {
	return tableGrid(headers, rows).Lines(Style{Unicode: true}, 0)
}

func tableGrid(headers []string, rows [][]string) Grid {
	cols := make([]Column, len(headers))
	for i, h := range headers {
		cols[i] = Column{Header: h}
	}
	return Grid{Columns: cols, Rows: rows}
}
