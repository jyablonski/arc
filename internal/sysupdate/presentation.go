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

const (
	defaultRenderWidth = 76
	// labelWidth is the shared first column of every row in a section: result
	// labels, plan package names and held packages all start their detail at
	// the same column.
	labelWidth = 20
)

type PackageChange struct {
	Name        string
	FromVersion string
	ToVersion   string
	Note        string
	SizeBytes   int64
	Replaces    string
	// Change is the AUR build-file classification ("pkgver + checksums").
	Change string
	// Attention marks a Change a person should read before approving.
	Attention bool
}

// Renderer draws arc update system in arc's shared grammar. Copies share one
// tally so the closing summary sees every warning, wherever it was raised.
type Renderer struct {
	Out        io.Writer
	Width      int
	ForceColor bool
	// Verbose prints warnings that are otherwise only counted.
	Verbose bool

	tally *tally
}

// tally is what the closing summary line reports.
type tally struct {
	upgraded int
	ignored  int
	warnings int
	hidden   int
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

func (r Renderer) style() output.Style {
	s := output.StyleFor(r.Out)
	s.Color = r.colorEnabled()
	return s
}

// NewRenderer returns a renderer whose copies share one summary tally. The
// layout is a fixed width so a long run reads consistently, narrowing only
// for a terminal that cannot fit it.
func NewRenderer(out io.Writer, verbose bool) Renderer {
	r := Renderer{Out: out, Verbose: verbose, tally: &tally{}}
	if w := output.StyleFor(out).Width; w > 0 && w < defaultRenderWidth {
		r.Width = w
	}
	return r
}

func (r Renderer) countUpgraded(n int) {
	if r.tally != nil {
		r.tally.upgraded += n
	}
}

func (r Renderer) countIgnored(n int) {
	if r.tally != nil {
		r.tally.ignored += n
	}
}

func (r Renderer) RunHeader(started time.Time) {
	st := r.style()
	_, _ = fmt.Fprintf(r.writer(), "%s\n%s\n\n",
		output.Justify(st.Bold("arc update system"), output.Timestamp(started), r.width()),
		st.Rule(r.width()))
}

// Section prints a section title with its summary right-aligned to the same
// edge as the run header and every row timing.
func (r Renderer) Section(title, summary string) {
	_, _ = fmt.Fprintln(r.writer(), output.Justify(r.style().Bold(title), summary, r.width()))
}

func (r Renderer) Result(label, detail string, duration time.Duration) {
	r.writeStatus(output.GlyphOK, label, detail, duration)
}

// InfoResult is a row that reports state without having changed anything.
func (r Renderer) InfoResult(label, detail string) {
	r.writeStatus(output.GlyphInfo, label, detail, 0)
}

func (r Renderer) Warning(message string) {
	if r.tally != nil {
		r.tally.warnings++
	}
	r.writeLine(output.GlyphWarn, message)
}

// QuietWarning records a warning that is normal for this workflow (e.g. AUR
// sources without PGP signatures). It is counted in the summary and printed
// only with --verbose.
func (r Renderer) QuietWarning(message string) {
	if r.tally != nil {
		r.tally.warnings++
		if !r.Verbose {
			r.tally.hidden++
			return
		}
	}
	r.writeLine(output.GlyphWarn, message)
}

func (r Renderer) Error(message string) {
	r.writeLine(output.GlyphFail, message)
}

func (r Renderer) Info(message string) {
	r.writeLine(output.GlyphInfo, message)
}

func (r Renderer) writeLine(g output.Glyph, message string) {
	_, _ = fmt.Fprintf(r.writer(), "  %s %s\n", r.style().Glyph(g), message)
}

func (r Renderer) DiffPackage(name, summary string) {
	r.Blank()
	heading := "diff · " + name
	if summary != "" {
		heading += " · " + summary
	}
	_, _ = fmt.Fprintf(r.writer(), "    %s\n", r.ansi("\x1b[1;36m", heading))
}

func (r Renderer) DiffFile(name string) {
	_, _ = fmt.Fprintf(r.writer(), "    %s\n", r.ansi("\x1b[1m", name))
}

func (r Renderer) DiffLine(line string) {
	style := ""
	switch {
	case strings.HasPrefix(line, "+"):
		style = "\x1b[32m"
	case strings.HasPrefix(line, "-"):
		style = "\x1b[31m"
	case strings.HasPrefix(line, "@@"):
		style = "\x1b[36m"
	}
	_, _ = fmt.Fprintf(r.writer(), "    %s\n", r.ansi(style, line))
}

func (r Renderer) ansi(style, value string) string {
	if style == "" || !r.colorEnabled() {
		return value
	}
	return style + value + "\x1b[0m"
}

func (r Renderer) colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if r.ForceColor {
		return true
	}
	f, ok := r.Out.(*os.File)
	return ok && isTerminal(f)
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

// Plan lists pending changes and held packages with the same prefix and label
// column as result rows, so versions start where result details start; the
// remaining columns are a grid so every arrow, age and verdict lines up.
func (r Renderer) Plan(changes []PackageChange, held ...ignoredPackage) {
	if len(changes) == 0 && len(held) == 0 {
		return
	}
	st := r.style()
	changes = append([]PackageChange(nil), changes...)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })

	// The plan's name column grows to its longest name so an overlong AUR
	// name never shifts the columns after it.
	fromWidth, nameWidth := 0, labelWidth
	for _, c := range changes {
		fromWidth = max(fromWidth, visibleRunes(displayVersion(c.FromVersion)))
		nameWidth = max(nameWidth, visibleRunes(c.Name)+2)
	}
	for _, h := range held {
		nameWidth = max(nameWidth, visibleRunes(h.Name)+2)
	}
	pad := func(name string) string { return name + strings.Repeat(" ", nameWidth-visibleRunes(name)) }
	prefixes := make([]string, 0, len(changes)+len(held))
	grid := output.Grid{Columns: []output.Column{{}, {}, {}}, NoHeader: true}
	for _, c := range changes {
		from := displayVersion(c.FromVersion)
		version := from + strings.Repeat(" ", fromWidth-visibleRunes(from)) + " " + st.Arrow() + " " + c.ToVersion
		change := c.Change
		if c.Attention {
			change = st.Yellow(change)
		}
		prefixes = append(prefixes, "    "+pad(c.Name))
		grid.Rows = append(grid.Rows, []string{version, c.Note, change})
	}
	for _, h := range held {
		prefixes = append(prefixes, "  "+st.Glyph(output.GlyphInfo)+" "+pad(h.Name))
		grid.Rows = append(grid.Rows, []string{"held at " + h.Version, "", st.Faint("IgnorePkg")})
	}
	for i, line := range grid.Lines(st, 0) {
		_, _ = fmt.Fprintln(r.writer(), strings.TrimRight(prefixes[i]+line, " "))
	}
}

// Prompt asks for approval in arc's wording. defaultYes picks which answer
// an empty line means.
func (r Renderer) Prompt(label string, defaultYes bool) {
	choices := "[Y/n]"
	if !defaultYes {
		choices = "[y/N]"
	}
	_, _ = fmt.Fprintf(r.writer(), "  %s %s ", label, r.style().Cyan(choices))
}

// EndPrompt finishes a prompt line. A terminal already echoed the newline
// with the answer; anything else (piped input) needs one written.
func (r Renderer) EndPrompt() {
	f, ok := r.Out.(*os.File)
	if !ok || !isTerminal(f) {
		r.Blank()
	}
}

func (r Renderer) PackageResult(change PackageChange, duration time.Duration) {
	detail := change.ToVersion
	if change.FromVersion == "" {
		detail += "  installed"
	}
	r.Result(change.Name, detail, duration)
}

func (r Renderer) LogPath(path string) {
	if path == "" {
		return
	}
	r.InfoResult("log", path)
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

// Footer closes the run with a rule and the one line a returning user reads.
func (r Renderer) Footer(elapsed time.Duration, extra ...string) {
	st := r.style()
	var parts []string
	t := r.tally
	if t == nil {
		t = &tally{}
	}
	parts = append(parts, fmt.Sprintf("%d upgraded", t.upgraded))
	if t.ignored > 0 {
		parts = append(parts, fmt.Sprintf("%d ignored", t.ignored))
	}
	for _, e := range extra {
		if e != "" {
			parts = append(parts, e)
		}
	}
	if t.warnings > 0 {
		w := st.Yellow(output.Count(t.warnings, "warning", "warnings"))
		if t.hidden > 0 {
			w += " " + st.Faint("(arc update system -v)")
		}
		parts = append(parts, w)
	}
	parts = append(parts, output.Duration(elapsed))
	_, _ = fmt.Fprintf(r.writer(), "\n%s\n%s\n", st.Rule(r.width()), strings.Join(parts, st.Sep()))
}

// writeStatus renders "  G label  detail" with any duration right-aligned to
// the frame edge shared with section summaries.
func (r Renderer) writeStatus(g output.Glyph, label, detail string, duration time.Duration) {
	left := "  " + r.style().Glyph(g) + " " + padLabel(label) + detail
	if duration > 0 {
		left = output.Justify(strings.TrimRight(left, " "), output.Duration(duration), r.width())
	}
	_, _ = fmt.Fprintln(r.writer(), strings.TrimRight(left, " "))
}

// writeStatusHint is writeStatus with a faint hint right-aligned where a
// timing would go.
func (r Renderer) writeStatusHint(g output.Glyph, label, detail, hint string) {
	left := "  " + r.style().Glyph(g) + " " + padLabel(label) + detail
	_, _ = fmt.Fprintln(r.writer(), output.Justify(left, hint, r.width()))
}

// padLabel pads to the shared label column, keeping a gutter after names that
// overflow it.
func padLabel(label string) string {
	if n := visibleRunes(label); n < labelWidth {
		return label + strings.Repeat(" ", labelWidth-n)
	}
	return label + "  "
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
