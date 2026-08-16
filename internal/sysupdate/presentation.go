package sysupdate

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/mattn/go-isatty"
)

const defaultRenderWidth = 76

type PackageChange struct {
	Name        string
	FromVersion string
	ToVersion   string
	Note        string
	SizeBytes   int64
	Replaces    string
}

type Renderer struct {
	Out   io.Writer
	Width int
}

func (r Renderer) writer() io.Writer {
	if r.Out == nil {
		return io.Discard
	}
	return r.Out
}

func (r Renderer) width() int {
	if r.Width < 40 {
		return defaultRenderWidth
	}
	return r.Width
}

func (r Renderer) RunHeader(started time.Time) {
	left := "arc update system"
	right := started.Format("2006-01-02 15:04:05")
	gap := max(1, r.width()-visibleRunes(left)-visibleRunes(right))
	_, _ = fmt.Fprintf(r.writer(), "%s%s%s\n%s\n\n", left, strings.Repeat(" ", gap), right, strings.Repeat("─", r.width()))
}

func (r Renderer) Section(title, summary string) {
	if summary == "" {
		_, _ = fmt.Fprintf(r.writer(), "%s\n", title)
		return
	}
	gap := max(1, r.width()-visibleRunes(title)-visibleRunes(summary))
	_, _ = fmt.Fprintf(r.writer(), "%s%s%s\n", title, strings.Repeat(" ", gap), summary)
}

func (r Renderer) Result(label, detail string, duration time.Duration) {
	r.writeStatus("✓", label, detail, duration)
}

func (r Renderer) Warning(message string) {
	_, _ = fmt.Fprintf(r.writer(), "  ⚠ %s\n", message)
}

func (r Renderer) Error(message string) {
	_, _ = fmt.Fprintf(r.writer(), "  ✗ %s\n", message)
}

func (r Renderer) Info(message string) {
	_, _ = fmt.Fprintf(r.writer(), "  · %s\n", message)
}

// Progress writes a transient line only for an interactive terminal. The
// returned function clears it before a permanent result is rendered.
func (r Renderer) Progress(message string) func() {
	f, ok := r.Out.(*os.File)
	if !ok || !isTerminal(f) {
		return func() {}
	}
	line := "  … " + message
	if visibleRunes(line) > r.width() {
		runes := []rune(line)
		line = string(runes[:r.width()-1]) + "…"
	}
	_, _ = fmt.Fprintf(r.writer(), "\r\x1b[2K%s", line)
	return func() { _, _ = fmt.Fprint(r.writer(), "\r\x1b[2K") }
}

// ResetLine makes the next permanent result start at column zero after a
// subprocess wrote an unterminated prompt or carriage-return update.
func (r Renderer) ResetLine() {
	f, ok := r.Out.(*os.File)
	if ok && isTerminal(f) {
		_, _ = fmt.Fprint(r.writer(), "\r\x1b[2K")
	}
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func (r Renderer) Blank() {
	_, _ = fmt.Fprintln(r.writer())
}

func (r Renderer) Plan(changes []PackageChange) {
	if len(changes) == 0 {
		return
	}
	changes = append([]PackageChange(nil), changes...)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })

	nameWidth := 0
	fromWidth := 0
	toWidth := 0
	for _, change := range changes {
		nameWidth = max(nameWidth, len(change.Name))
		fromWidth = max(fromWidth, len(displayVersion(change.FromVersion)))
		toWidth = max(toWidth, len(change.ToVersion))
	}
	for _, change := range changes {
		_, _ = fmt.Fprintf(r.writer(), "  %-*s  %-*s → %s", nameWidth, change.Name, fromWidth, displayVersion(change.FromVersion), change.ToVersion)
		if change.Note != "" {
			_, _ = fmt.Fprintf(r.writer(), "%s  %s", strings.Repeat(" ", toWidth-len(change.ToVersion)), change.Note)
		}
		_, _ = fmt.Fprintln(r.writer())
	}

	if total := totalDownloadSize(changes); total > 0 {
		_, _ = fmt.Fprintf(r.writer(), "\n  download %s\n", output.Bytes(total))
	}
}

func (r Renderer) Prompt(label string) {
	_, _ = fmt.Fprintf(r.writer(), "\n  %s [Y/n] ", label)
}

func (r Renderer) PackageResult(change PackageChange) {
	detail := change.ToVersion
	if change.FromVersion == "" {
		detail += "  installed"
	}
	r.Result(change.Name, detail, 0)
}

func (r Renderer) LogPath(path string) {
	if path == "" {
		return
	}
	_, _ = fmt.Fprintf(r.writer(), "\n  log %s\n", path)
}

func (r Renderer) FailureTail(lines []string) {
	if len(lines) == 0 {
		return
	}
	_, _ = fmt.Fprintln(r.writer(), "\n  subprocess output:")
	for _, line := range lines {
		_, _ = fmt.Fprintf(r.writer(), "    %s\n", line)
	}
}

func (r Renderer) writeStatus(symbol, label, detail string, duration time.Duration) {
	const labelWidth = 25
	_, _ = fmt.Fprintf(r.writer(), "  %s %-*s", symbol, labelWidth, label)
	if detail != "" {
		_, _ = fmt.Fprint(r.writer(), detail)
	}
	if duration > 0 {
		d := formatDuration(duration)
		used := 4 + labelWidth + len(detail) + len(d)
		_, _ = fmt.Fprintf(r.writer(), "%s%s", strings.Repeat(" ", max(1, r.width()-used)), d)
	}
	_, _ = fmt.Fprintln(r.writer())
}

func visibleRunes(s string) int {
	return len([]rune(s))
}

func displayVersion(version string) string {
	if version == "" {
		return "—"
	}
	return version
}

func totalDownloadSize(changes []PackageChange) int64 {
	var total int64
	for _, change := range changes {
		total += change.SizeBytes
	}
	return total
}

func formatDuration(d time.Duration) string {
	if d < 100*time.Millisecond {
		return "<0.1s"
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}
