package output

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-isatty"
)

// This file is arc's shared output grammar: one rule character, a fixed glyph
// set, a title line, aligned grids, and a closing summary line. Commands build
// on it instead of hand-rolling alignment so every screen reads the same.

const (
	// FrameWidth is the minimum width of a title rule; content wider than this
	// widens the frame up to the terminal width.
	FrameWidth = 76
	// MaxFrameWidth caps how far wide terminals stretch a frame; past this a
	// flex column is truncated rather than making lines hard to scan.
	MaxFrameWidth = 120
	gutter        = "  "
)

// Glyph is a status marker with one fixed meaning across every command.
type Glyph int

const (
	// GlyphOK: done / in sync.
	GlyphOK Glyph = iota
	// GlyphWarn: needs a human.
	GlyphWarn
	// GlyphInfo: informational, no action implied.
	GlyphInfo
	// GlyphFail: failed or unusable.
	GlyphFail
	// GlyphDrift: present but differs from the source of truth.
	GlyphDrift
)

var glyphs = map[Glyph][2]string{
	GlyphOK:    {"✓", "+"},
	GlyphWarn:  {"⚠", "!"},
	GlyphInfo:  {"·", "-"},
	GlyphFail:  {"✗", "x"},
	GlyphDrift: {"≠", "~"},
}

var glyphStyles = map[Glyph]string{
	GlyphOK:    ansiGreen,
	GlyphWarn:  ansiYellow,
	GlyphInfo:  ansiFaint,
	GlyphFail:  ansiRed,
	GlyphDrift: ansiYellow,
}

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiFaint  = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// Style captures how a writer can be drawn to: whether it gets color, whether
// it can take box-drawing and glyph characters, and how wide it is. Pipes, CI
// logs, TERM=dumb, non-UTF-8 locales, and ARC_ASCII=1 get the plain path.
type Style struct {
	Color   bool
	Unicode bool
	// Width is the terminal width, or 0 when unknown (not a terminal).
	Width int
}

// StyleFor inspects w and the environment once.
func StyleFor(w io.Writer) Style {
	s := Style{Unicode: unicodeEnv(), Width: columnsEnv()}
	f, ok := w.(*os.File)
	if !ok || (!isatty.IsTerminal(f.Fd()) && !isatty.IsCygwinTerminal(f.Fd())) {
		return s
	}
	s.Color = os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	if s.Width == 0 {
		s.Width = terminalWidth(f)
	}
	return s
}

// columnsEnv honors an exported COLUMNS, which is how a width is usually
// communicated when output is piped or captured.
func columnsEnv() int {
	n, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func unicodeEnv() bool {
	if os.Getenv("ARC_ASCII") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" {
			v = strings.ToLower(v)
			return strings.Contains(v, "utf-8") || strings.Contains(v, "utf8")
		}
	}
	// No locale at all is common in containers and CI that still render UTF-8.
	return true
}

// Glyph renders g in its fixed color, or its ASCII stand-in.
func (s Style) Glyph(g Glyph) string {
	pair := glyphs[g]
	if !s.Unicode {
		return s.paint(glyphStyles[g], pair[1])
	}
	return s.paint(glyphStyles[g], pair[0])
}

// Sym returns the Unicode form of a decorative symbol or its ASCII fallback.
func (s Style) Sym(unicode, ascii string) string {
	if s.Unicode {
		return unicode
	}
	return ascii
}

// Arrow is the version-transition arrow.
func (s Style) Arrow() string { return s.Sym("→", "->") }

// Sep joins summary fragments.
func (s Style) Sep() string { return s.Sym(" · ", " | ") }

// Dash marks an intentionally empty cell so the grid never has holes.
func (s Style) Dash() string { return s.Faint(s.Sym("—", "-")) }

func (s Style) Faint(v string) string  { return s.paint(ansiFaint, v) }
func (s Style) Bold(v string) string   { return s.paint(ansiBold, v) }
func (s Style) Cyan(v string) string   { return s.paint(ansiCyan, v) }
func (s Style) Green(v string) string  { return s.paint(ansiGreen, v) }
func (s Style) Yellow(v string) string { return s.paint(ansiYellow, v) }
func (s Style) Red(v string) string    { return s.paint(ansiRed, v) }

func (s Style) paint(sgr, v string) string {
	if !s.Color || sgr == "" || v == "" {
		return v
	}
	return sgr + v + ansiReset
}

// Truncate shortens s to width visible runes, ending in an ellipsis. Escape
// sequences are preserved (and terminated) so a truncated colored cell keeps
// its color instead of bleeding into the rest of the line.
func (s Style) Truncate(v string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(v) <= width {
		return v
	}
	ell := s.Sym("…", "~")
	keep := width
	if width > len([]rune(ell)) {
		keep = width - len([]rune(ell))
	} else {
		ell = ""
	}

	var b strings.Builder
	shown, styled := 0, false
	for i := 0; i < len(v); {
		if loc := ansiPattern.FindStringIndex(v[i:]); loc != nil && loc[0] == 0 {
			b.WriteString(v[i : i+loc[1]])
			styled = true
			i += loc[1]
			continue
		}
		r, size := utf8.DecodeRuneInString(v[i:])
		if shown == keep {
			break
		}
		b.WriteRune(r)
		shown++
		i += size
	}
	b.WriteString(ell)
	if styled {
		b.WriteString(ansiReset)
	}
	return b.String()
}

// frameWidth is the width every title rule, right-aligned value and footer
// shares, so right edges line up across a whole screen.
func (s Style) frameWidth(content int) int {
	w := min(max(FrameWidth, content), s.maxWidth())
	if s.Width > 0 && s.Width < w {
		w = s.Width
	}
	return w
}

// maxWidth is the widest a line may render: the terminal, capped at
// MaxFrameWidth. Non-terminals get the cap so piped output stays bounded.
func (s Style) maxWidth() int {
	if s.Width > 0 {
		return min(s.Width, MaxFrameWidth)
	}
	return MaxFrameWidth
}

// Justify places left and right on one line with right ending at width. When
// they do not fit, right follows left after a gutter instead of wrapping.
func Justify(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := width - visibleWidth(left) - visibleWidth(right)
	if gap < 2 {
		return left + gutter + right
	}
	return left + strings.Repeat(" ", gap) + right
}

// Rule returns a horizontal rule of the given width.
func (s Style) Rule(width int) string {
	return s.Faint(strings.Repeat(s.Sym("─", "-"), width))
}

// Align positions cell content within its column.
type Align int

const (
	AlignLeft Align = iota
	AlignRight
	AlignCenter
)

// Column describes one grid column. A Flex column absorbs the remaining frame
// width and is truncated to fit; at most one column should be Flex.
type Column struct {
	Header string
	Align  Align
	Flex   bool
}

// Grid is arc's canonical table: lowercase faint headers aligned exactly like
// their cells, a two-space gutter, no underline rule (the title rule already
// frames the table), and right-trimmed lines.
type Grid struct {
	Columns []Column
	Rows    [][]string
	// Indent prefixes every line (section tables inside update system).
	Indent string
	// NoHeader omits the header row (callout blocks).
	NoHeader bool
}

// Width reports the natural (untruncated) width of the grid.
func (g Grid) Width() int {
	widths := g.widths()
	total := visibleWidth(g.Indent)
	for i, w := range widths {
		if i > 0 {
			total += len(gutter)
		}
		total += w
	}
	return total
}

func (g Grid) widths() []int {
	widths := make([]int, len(g.Columns))
	for i, c := range g.Columns {
		widths[i] = visibleWidth(c.Header)
	}
	for _, row := range g.Rows {
		for i, cell := range row {
			if i < len(widths) {
				widths[i] = max(widths[i], visibleWidth(cell))
			}
		}
	}
	return widths
}

// Lines renders the grid. maxWidth > 0 truncates the Flex column so no line
// exceeds it.
func (g Grid) Lines(s Style, maxWidth int) []string {
	widths := g.widths()
	if maxWidth > 0 {
		if over := g.Width() - maxWidth; over > 0 {
			for i, c := range g.Columns {
				if c.Flex {
					widths[i] = max(visibleWidth(c.Header), widths[i]-over)
				}
			}
		}
	}

	lines := make([]string, 0, len(g.Rows)+1)
	render := func(cells []string, header bool) {
		var b strings.Builder
		b.WriteString(g.Indent)
		for i, c := range g.Columns {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if c.Flex {
				cell = s.Truncate(cell, widths[i])
			}
			if header {
				cell = s.Faint(cell)
			}
			if i > 0 {
				b.WriteString(gutter)
			}
			b.WriteString(pad(cell, widths[i], c.Align))
		}
		line := strings.TrimRight(b.String(), " ")
		// A terminal too narrow for even the fixed columns still gets whole
		// lines: clamp rather than letting rows wrap past the frame.
		if maxWidth > 0 {
			line = s.Truncate(line, maxWidth)
		}
		lines = append(lines, line)
	}

	if !g.NoHeader {
		headers := make([]string, len(g.Columns))
		for i, c := range g.Columns {
			headers[i] = strings.ToLower(c.Header)
		}
		render(headers, true)
	}
	for _, row := range g.Rows {
		render(row, false)
	}
	return lines
}

func pad(cell string, width int, align Align) string {
	gap := width - visibleWidth(cell)
	if gap <= 0 {
		return cell
	}
	switch align {
	case AlignRight:
		return strings.Repeat(" ", gap) + cell
	case AlignCenter:
		left := gap / 2
		return strings.Repeat(" ", left) + cell + strings.Repeat(" ", gap-left)
	default:
		return cell + strings.Repeat(" ", gap)
	}
}

// Screen is one command's human output: a title line with right-aligned
// metadata, a rule, a body, and a closing summary line. Every right edge in
// the screen ends at the same column.
type Screen struct {
	W     io.Writer
	Style Style
	Title string
	// Meta is right-aligned on the title line (timestamp, counts).
	Meta string
	body []string
}

// Line appends a body line.
func (sc *Screen) Line(line string) { sc.body = append(sc.body, line) }

// Blank appends an empty body line.
func (sc *Screen) Blank() { sc.body = append(sc.body, "") }

// Grid appends a grid, truncating its Flex column to the terminal width.
func (sc *Screen) Grid(g Grid) {
	sc.body = append(sc.body, g.Lines(sc.Style, sc.Style.maxWidth())...)
}

// Note is one line of a callout block under a table: a state, what it
// applies to, and the command that resolves it.
type Note struct {
	Glyph Glyph
	// Label is the state ("1 unmanaged"); an empty label continues the
	// previous note's group without repeating the glyph.
	Label  string
	Detail string
	Hint   string
}

// Notes appends callouts with label, detail and hint each in aligned columns.
// Unlike a table, notes are not clamped to the frame: a note's hint is the
// command that resolves it, so on a narrow terminal it should wrap rather
// than lose its tail.
func (sc *Screen) Notes(notes []Note) {
	g := Grid{Columns: []Column{{}, {}, {}}, NoHeader: true}
	for _, n := range notes {
		first := " "
		if n.Label != "" {
			first = sc.Style.Glyph(n.Glyph)
		}
		g.Rows = append(g.Rows, []string{first + " " + n.Label, n.Detail, sc.Style.Faint(n.Hint)})
	}
	sc.body = append(sc.body, g.Lines(sc.Style, 0)...)
}

// Width is the shared right edge for this screen.
func (sc *Screen) Width() int {
	content := visibleWidth(sc.Title) + len(gutter) + visibleWidth(sc.Meta)
	for _, l := range sc.body {
		content = max(content, visibleWidth(l))
	}
	return sc.Style.frameWidth(content)
}

// Flush writes the title, rule, body, and the closing summary line. An empty
// summary omits the closing line.
func (sc *Screen) Flush(summary string) {
	width := sc.Width()
	_, _ = fmt.Fprintln(sc.W, Justify(sc.Style.Bold(sc.Title), sc.Style.Faint(sc.Meta), width))
	_, _ = fmt.Fprintln(sc.W, sc.Style.Rule(width))
	_, _ = fmt.Fprintln(sc.W)
	for _, l := range sc.body {
		_, _ = fmt.Fprintln(sc.W, l)
	}
	if summary != "" {
		if len(sc.body) > 0 {
			_, _ = fmt.Fprintln(sc.W)
		}
		_, _ = fmt.Fprintln(sc.W, summary)
	}
}

// TildePath abbreviates the home directory to ~ for display.
func TildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(os.PathSeparator)); ok {
		return "~/" + rest
	}
	return path
}

// Timestamp is the one clock format arc prints: 24-hour, local time.
func Timestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// Duration is the one elapsed-time format: "<0.1s", "0.7s", "38.2s",
// "1m 04s", "2h 05m".
func Duration(d time.Duration) string {
	switch {
	case d < 100*time.Millisecond:
		return "<0.1s"
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		d = d.Round(time.Second)
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		d = d.Round(time.Minute)
		return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// Age is the one compact "how long ago / how long until" format: "1m",
// "20h 36m", "3d 22h", "5d".
func Age(d time.Duration) string {
	d = max(d, 0).Round(time.Minute)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(1, int(d.Minutes())))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		days, hours := int(d.Hours())/24, int(d.Hours())%24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// Plural picks the singular or plural noun for n.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// Count renders "n noun" with the right plural.
func Count(n int, singular, plural string) string {
	return fmt.Sprintf("%d %s", n, Plural(n, singular, plural))
}
