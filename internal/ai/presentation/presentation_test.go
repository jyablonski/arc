package presentation

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func renderUsage(t *testing.T, agg ai.AggregateReport, opts UsageOptions) string {
	t.Helper()
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	var buf bytes.Buffer
	PrintUsage(&buf, agg, opts)
	return buf.String()
}

func TestPrintUsage_oneTableAcrossProviders(t *testing.T) {
	now := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	reset := now.Add(3 * time.Hour)
	out := renderUsage(t, ai.AggregateReport{
		FetchedAt: now,
		Providers: []ai.ProviderResult{
			{Name: "claude", OK: true, Report: ai.UsageReport{Windows: []ai.UsageWindow{
				{Label: "5 hour", PercentUsed: 0},
				{Label: "7 day (all models)", PercentUsed: 3, ResetsAt: &reset},
			}}},
			{Name: "codex", OK: true, Report: ai.UsageReport{Windows: []ai.UsageWindow{{Label: "weekly", PercentUsed: 1, ResetsAt: &reset}}}},
		},
	}, UsageOptions{Now: now})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.True(t, strings.HasPrefix(lines[0], "arc ai usage"))
	require.True(t, strings.HasSuffix(lines[0], "2026-05-06 14:00:00"))
	require.Equal(t, "provider  window  used              left  resets", lines[3])
	require.Equal(t, "claude    5 hour  ░░░░░░░░░░░░░░░░  100%  not started", lines[4])
	require.Equal(t, "claude    7 day   ▓░░░░░░░░░░░░░░░   97%  in 3h 0m", lines[5], "bar fills with what is consumed")
	require.Equal(t, "codex     7 day   ▓░░░░░░░░░░░░░░░   99%  in 3h 0m", lines[6])
	require.Equal(t, "✓ all windows clear · tightest is claude 7 day at 97% left", lines[len(lines)-1])
}

func TestPrintUsage_short(t *testing.T) {
	out := renderUsage(t, ai.AggregateReport{Providers: []ai.ProviderResult{
		{Name: "claude", OK: true, Report: ai.UsageReport{Windows: []ai.UsageWindow{{Label: "5 hour", PercentUsed: 3}, {Label: "7 day (all models)", PercentUsed: 1}}}},
	}}, UsageOptions{Short: true})
	require.Equal(t, "✓ 2/2 windows clear · tightest claude 5 hour 97%\n", out)
}

func TestPrintUsage_lowWindowWarns(t *testing.T) {
	out := renderUsage(t, ai.AggregateReport{Providers: []ai.ProviderResult{
		{Name: "claude", OK: true, Report: ai.UsageReport{Windows: []ai.UsageWindow{{Label: "5 hour", PercentUsed: 92.5}}}},
		{Name: "codex", OK: false, Error: "offline", Hint: "install codex"},
	}}, UsageOptions{Short: true})
	require.Equal(t, "⚠ 1 window low · tightest claude 5 hour 7.5% · codex unavailable\n", out)
}

func TestPrintUsage_providerErrors(t *testing.T) {
	out := renderUsage(t, ai.AggregateReport{Providers: []ai.ProviderResult{
		{Name: "codex", OK: false, Error: "offline", Hint: "install codex"},
		{Name: "cursor", OK: true},
	}}, UsageOptions{})
	// Provider, message and hint each get a column, the same callout shape
	// `arc mcp list` and `arc skills list` use.
	require.Contains(t, out, "✗ codex   offline                    install codex")
	require.Contains(t, out, "· cursor  no usage windows returned")
	require.Contains(t, out, "✗ no usage available")
}

func TestLeftPercent(t *testing.T) {
	require.Equal(t, "100%", leftPercent(0))
	require.Equal(t, "99%", leftPercent(0.4), "any consumption reads below 100%")
	require.Equal(t, "72%", leftPercent(28))
	require.Equal(t, "7.5%", leftPercent(92.5))
	require.Equal(t, "0.0%", leftPercent(150))
}
