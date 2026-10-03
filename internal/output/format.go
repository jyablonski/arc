package output

import (
	"fmt"
	"os"
	"regexp"
)

// ansiPattern matches SGR escape sequences (e.g. color codes) so table layout
// can measure and pad cells by their visible width rather than byte length.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleWidth returns the display width of s with ANSI escape sequences removed.
func visibleWidth(s string) int {
	return len([]rune(ansiPattern.ReplaceAllString(s, "")))
}

// live is the stdout stream behind the package-level helpers, for commands
// and internal packages that only ever report to the terminal. Code that can
// also run under --json takes a *Stream instead (see Stream).
var live = &Stream{}

// Live returns the stdout stream the package-level helpers write to.
func Live() *Stream { return live }

func Title(title, meta string)         { live.Title(title, meta) }
func Section(title string)             { live.Section(title) }
func Summary(g Glyph, parts ...string) { live.Summary(g, parts...) }
func Success(s string)                 { live.Success(s) }
func Error(s string)                   { live.Error(s) }
func Info(s string)                    { live.Info(s) }
func Warning(s string)                 { live.Warning(s) }

// Failure writes a command's returned error to stderr as the closing line,
// set off from any steps above it.
func Failure(err error) {
	if live.body {
		fmt.Fprintln(os.Stderr)
	}
	NewStream(os.Stderr).Error(err.Error())
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
