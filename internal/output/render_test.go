package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrid_alignsHeadersWithCells(t *testing.T) {
	g := Grid{
		Columns: []Column{
			{Header: "name"},
			{Header: "msgs", Align: AlignRight},
			{Header: "claude", Align: AlignCenter},
		},
		Rows: [][]string{
			{"ci-fix", "204", "✓"},
			{"a", "7", "·"},
		},
	}
	lines := g.Lines(Style{Unicode: true}, 0)
	require.Equal(t, []string{
		"name    msgs  claude",
		"ci-fix   204    ✓",
		"a          7    ·",
	}, lines)
}

func TestGrid_truncatesFlexColumnToWidth(t *testing.T) {
	g := Grid{
		Columns: []Column{{Header: "id"}, {Header: "title", Flex: true}},
		Rows:    [][]string{{"01", "a very long title that will not fit"}},
	}
	lines := g.Lines(Style{Unicode: true}, 20)
	for _, l := range lines {
		assert.LessOrEqual(t, visibleWidth(l), 20, l)
	}
	assert.True(t, strings.HasSuffix(lines[1], "…"))
}

func TestGrid_ignoresANSIWhenAligning(t *testing.T) {
	s := Style{Color: true, Unicode: true}
	g := Grid{
		Columns: []Column{{Header: "p"}, {Header: "n", Align: AlignRight}},
		Rows:    [][]string{{s.Cyan("claude"), "1"}, {"codex", "22"}},
	}
	lines := g.Lines(Style{Unicode: true}, 0)
	plain := ansiPattern.ReplaceAllString(lines[1], "")
	assert.Equal(t, len(plain), len(lines[2]))
}

func TestScreen_sharesOneRightEdge(t *testing.T) {
	var buf bytes.Buffer
	sc := &Screen{W: &buf, Style: Style{Unicode: true}, Title: "arc demo", Meta: "2026-09-19 14:04:49"}
	sc.Line("body")
	sc.Flush("✓ done")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 6)
	assert.Equal(t, FrameWidth, visibleWidth(lines[0]))
	assert.Equal(t, FrameWidth, visibleWidth(lines[1]))
	assert.Equal(t, "✓ done", lines[5])
}

func TestStyle_asciiFallback(t *testing.T) {
	t.Setenv("ARC_ASCII", "1")
	s := StyleFor(&bytes.Buffer{})
	assert.False(t, s.Unicode)
	assert.Equal(t, "+", s.Glyph(GlyphOK))
	assert.Equal(t, "->", s.Arrow())
	assert.Equal(t, "---", s.Rule(3))
}

func TestJustify(t *testing.T) {
	assert.Equal(t, "ab   cd", Justify("ab", "cd", 7))
	assert.Equal(t, "abcdef  gh", Justify("abcdef", "gh", 5), "overflow falls back to a gutter")
}

func TestDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		50 * time.Millisecond:    "<0.1s",
		700 * time.Millisecond:   "0.7s",
		38200 * time.Millisecond: "38.2s",
		64 * time.Second:         "1m 04s",
		125 * time.Minute:        "2h 05m",
	} {
		assert.Equal(t, want, Duration(d))
	}
}

func TestAge(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second:              "1m",
		20*time.Hour + 36*time.Minute: "20h 36m",
		94 * time.Hour:                "3d 22h",
		120 * time.Hour:               "5d",
	} {
		assert.Equal(t, want, Age(d))
	}
}

func TestStyleFor_honorsCOLUMNS(t *testing.T) {
	t.Setenv("COLUMNS", "60")
	assert.Equal(t, 60, StyleFor(&bytes.Buffer{}).Width)
	t.Setenv("COLUMNS", "not-a-number")
	assert.Equal(t, 0, StyleFor(&bytes.Buffer{}).Width)
}

func TestStyle_TruncateKeepsColorAndTerminates(t *testing.T) {
	s := Style{Color: true, Unicode: true}
	got := s.Truncate(s.Cyan("abcdefgh"), 4)
	assert.Equal(t, 4, visibleWidth(got))
	assert.True(t, strings.HasPrefix(got, ansiCyan), got)
	assert.True(t, strings.HasSuffix(got, ansiReset), got)
	assert.Contains(t, got, "abc…")
}

func TestGrid_clampsRowsToMaxWidth(t *testing.T) {
	g := Grid{
		Columns: []Column{{Header: "aaaaaaaaaa"}, {Header: "bbbbbbbbbb"}},
		Rows:    [][]string{{"1111111111", "2222222222"}},
	}
	// Narrower than the fixed columns: rows are clamped, never wrapped.
	for _, line := range g.Lines(Style{Unicode: true}, 12) {
		assert.LessOrEqual(t, visibleWidth(line), 12, line)
	}
}
