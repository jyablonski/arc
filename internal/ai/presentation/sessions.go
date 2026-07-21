package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

type SessionsPrintOptions struct {
	ShowResume bool
}

func PrintSessions(report ai.SessionReport, opts SessionsPrintOptions) {
	output.SectionAccent("AI sessions", providerAccent("claude"))
	if len(report.Sessions) == 0 {
		output.Info("no local sessions found")
		return
	}

	headers := []string{"provider", "session", "age", "model", "msgs", "tokens", "project", "title"}
	rows := make([][]string, 0, len(report.Sessions))
	for _, s := range report.Sessions {
		accent := providerAccent(s.Provider)
		rows = append(rows, []string{
			accent.Sprint(s.Provider),
			shortSessionID(s.SessionID),
			relativeAge(s.LastAt),
			s.Model,
			fmt.Sprintf("%d", s.Messages),
			humanizeCount(s.Tokens.Total()),
			projectLabel(s.Project),
			truncate(s.Title, 60),
		})
	}
	output.Table(headers, rows)

	if opts.ShowResume {
		output.Print("")
		for _, s := range report.Sessions {
			if cmd := ResumeCommand(s); cmd != "" {
				output.Print(fmt.Sprintf("  %s  %s", shortSessionID(s.SessionID), cmd))
			}
		}
	}
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
		return "-"
	}
	return filepath.Base(project)
}

func relativeAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return compactDurationRemain(max(time.Since(t), 0))
}

func truncate(s string, width int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}
