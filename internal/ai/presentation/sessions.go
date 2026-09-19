package presentation

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

type SessionsPrintOptions struct {
	ShowResume bool
	Now        time.Time
}

// PrintSessions renders the session list led by age (the sort key), with
// empty cells marked, the title filling the terminal, and a footer that says
// what was left out and how to see it.
func PrintSessions(w io.Writer, report ai.SessionReport, opts SessionsPrintOptions) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	style := output.StyleFor(w)
	meta := output.Timestamp(report.FetchedAt)
	if report.FetchedAt.IsZero() {
		meta = output.Timestamp(opts.Now)
	}
	sc := &output.Screen{W: w, Style: style, Title: "arc ai sessions", Meta: meta}

	var total int64
	grid := output.Grid{Columns: []output.Column{
		{Header: "age"},
		{Header: "session"},
		{Header: "provider"},
		{Header: "model"},
		{Header: "msgs", Align: output.AlignRight},
		{Header: "tokens", Align: output.AlignRight},
		{Header: "project"},
		{Header: "title", Flex: true},
	}}
	for _, s := range report.Sessions {
		total += s.Tokens.Total()
		grid.Rows = append(grid.Rows, []string{
			orDash(style, relativeAge(s.LastAt, opts.Now)),
			style.Cyan(shortSessionID(s.SessionID)),
			s.Provider,
			orDash(style, shortModel(s.Provider, s.Model)),
			fmt.Sprintf("%d", s.Messages),
			humanizeCount(s.Tokens.Total()),
			orDash(style, projectLabel(s.Project)),
			orDash(style, strings.TrimSpace(s.Title)),
		})
	}
	if len(report.Sessions) > 0 {
		sc.Grid(grid)
	}
	var notes []output.Note
	for _, p := range report.Providers {
		if !p.OK {
			notes = append(notes, output.Note{Glyph: output.GlyphFail, Label: p.Name, Detail: p.Error, Hint: p.Hint})
		}
	}
	if len(notes) > 0 {
		if len(report.Sessions) > 0 {
			sc.Blank()
		}
		sc.Notes(notes)
	}

	if opts.ShowResume && len(report.Sessions) > 0 {
		sc.Blank()
		for _, s := range report.Sessions {
			if cmd := ResumeCommand(s); cmd != "" {
				sc.Line(style.Cyan(shortSessionID(s.SessionID)) + "  " + cmd)
			}
		}
	}

	sc.Flush(sessionsFooter(style, report, total))
}

func sessionsFooter(style output.Style, report ai.SessionReport, total int64) string {
	shown := len(report.Sessions)
	if shown == 0 && report.HiddenAutomated == 0 {
		return style.Glyph(output.GlyphInfo) + " no local sessions found"
	}
	var parts []string
	if shown < report.Matched {
		parts = append(parts, fmt.Sprintf("%d of %d sessions (--limit 0 for all)", shown, report.Matched))
	} else {
		parts = append(parts, output.Count(shown, "session", "sessions"))
	}
	if report.HiddenAutomated > 0 {
		parts = append(parts, fmt.Sprintf("%d auto-review hidden (--all)", report.HiddenAutomated))
	}
	if total > 0 {
		parts = append(parts, humanizeCount(total)+" tokens")
	}
	return style.Faint(strings.Join(parts, style.Sep()))
}

// shortModel drops the provider prefix the provider column already states.
func shortModel(provider, model string) string {
	return strings.TrimPrefix(model, provider+"-")
}

func orDash(style output.Style, v string) string {
	if v == "" {
		return style.Dash()
	}
	return v
}

// ResumeCommand renders the CLI invocation that reopens a session in its own
// tool. Empty when the provider or resume id is unknown.
func ResumeCommand(s ai.SessionSummary) string {
	id := s.ResumeID
	if id == "" {
		id = s.SessionID
	}
	if id == "" {
		return ""
	}
	switch s.Provider {
	case "claude":
		return fmt.Sprintf("claude --resume %s", id)
	case "codex":
		return fmt.Sprintf("codex resume %s", id)
	default:
		return ""
	}
}

func projectLabel(project string) string {
	if project == "" {
		return ""
	}
	return filepath.Base(project)
}

func relativeAge(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	return output.Age(now.Sub(t))
}
