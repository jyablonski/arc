package presentation

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestPrintSessions_leadsWithAgeAndReportsWhatIsHidden(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	now := time.Date(2026, 9, 19, 14, 5, 1, 0, time.UTC)
	report := ai.SessionReport{
		FetchedAt: now,
		Providers: []ai.SessionProviderResult{{Name: "claude", OK: true}},
		Sessions: []ai.SessionSummary{
			{Provider: "codex", SessionID: "01a0ba5e", Model: "gpt-5.6-luna", Messages: 204,
				Tokens: ai.TokenBreakdown{Input: 17_600_000}, Project: "/home/x/jyablonski_praq",
				Title: "can we review projects", LastAt: now.Add(-time.Minute)},
			{Provider: "claude", SessionID: "9e2ba0a2", Model: "claude-opus-5", Messages: 61,
				Tokens: ai.TokenBreakdown{Input: 12_000_000}, LastAt: now.Add(-116 * time.Hour)},
		},
		Matched:         20,
		HiddenAutomated: 12,
	}
	var buf bytes.Buffer
	PrintSessions(&buf, report, SessionsPrintOptions{Now: now})
	lines := strings.Split(buf.String(), "\n")

	require.True(t, strings.HasPrefix(lines[0], "arc ai sessions"))
	require.Equal(t, "age     session   provider  model         msgs  tokens  project          title", lines[3])
	require.Equal(t, "1m      01a0ba5e  codex     gpt-5.6-luna   204   17.6M  jyablonski_praq  can we review projects", lines[4])
	// The provider prefix is dropped from the model, and empty cells are marked.
	require.Equal(t, "4d 20h  9e2ba0a2  claude    opus-5          61   12.0M  —                —", lines[5])
	require.Contains(t, buf.String(), "2 of 20 sessions (--limit 0 for all) · 12 auto-review hidden (--all) · 29.6M tokens")
}

func TestPrintSessions_emptyAndProviderFailure(t *testing.T) {
	var buf bytes.Buffer
	PrintSessions(&buf, ai.SessionReport{
		Providers: []ai.SessionProviderResult{{Name: "codex", Error: "no logs", Hint: "install codex"}},
	}, SessionsPrintOptions{Now: time.Now()})
	require.Contains(t, buf.String(), "✗ codex  no logs  install codex")
	require.Contains(t, buf.String(), "no local sessions found")
}

func TestShortModel(t *testing.T) {
	require.Equal(t, "opus-5", shortModel("claude", "claude-opus-5"))
	require.Equal(t, "gpt-5.6-luna", shortModel("codex", "gpt-5.6-luna"))
}
