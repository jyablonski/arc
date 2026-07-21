package presentation

import (
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestHealthSection(t *testing.T) {
	cases := []struct {
		name        string
		check       ai.HealthCheck
		wantSection string
		wantLabel   string
	}{
		{"auth groups under provider", ai.HealthCheck{Category: "auth", Name: "claude"}, "claude", "auth"},
		{"tooling groups under provider", ai.HealthCheck{Category: "tooling", Name: "codex"}, "codex", "tooling"},
		{"pricing is machine-wide", ai.HealthCheck{Category: "pricing", Name: "pricing"}, "local", "pricing"},
		{"config is machine-wide", ai.HealthCheck{Category: "config", Name: "skills"}, "local", "skills"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			section, label := healthSection(tc.check)
			require.Equal(t, tc.wantSection, section)
			require.Equal(t, tc.wantLabel, label)
		})
	}
}

func TestPrintHealth_allOK_ordersProvidersAndHidesHints(t *testing.T) {
	// Deliberately out of order; the printer must sort claude → codex → cursor → local.
	report := ai.HealthReport{Checks: []ai.HealthCheck{
		{Category: "config", Name: "skills", Status: ai.HealthOK, Detail: "6 skills linked"},
		{Category: "auth", Name: "cursor", Status: ai.HealthOK, Detail: "cursor token ok"},
		{Category: "auth", Name: "codex", Status: ai.HealthOK, Detail: "codex token ok", Hint: "hint-should-not-render"},
		{Category: "auth", Name: "claude", Status: ai.HealthOK, Detail: "claude token ok"},
	}}

	out := captureStdout(t, func() { PrintHealth(report) })

	require.Contains(t, out, "status")
	require.Contains(t, out, "provider")
	// row ordering: claude before codex before cursor before local
	require.Less(t, strings.Index(out, "claude token ok"), strings.Index(out, "codex token ok"))
	require.Less(t, strings.Index(out, "codex token ok"), strings.Index(out, "cursor token ok"))
	require.Less(t, strings.Index(out, "cursor token ok"), strings.Index(out, "6 skills linked"))
	// no failures → no hint footer, even though an OK check carries a hint
	require.NotContains(t, out, "hint-should-not-render")
	require.NotContains(t, out, "✗")
	require.NotContains(t, out, "!")
}

func TestPrintHealth_brokenChecks_showGlyphsDetailsAndHints(t *testing.T) {
	report := ai.HealthReport{Checks: []ai.HealthCheck{
		{Category: "auth", Name: "claude", Status: ai.HealthOK, Detail: "token valid for 5h", Hint: "ok-hint-hidden"},
		{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: "no access token in auth.json", Hint: "run 'codex login'"},
		{Category: "config", Name: "skills", Status: ai.HealthWarn, Detail: "2 skill link(s) dangling", Hint: "run 'arc skills sync'"},
	}}

	out := captureStdout(t, func() { PrintHealth(report) })

	// status glyphs (color is disabled in captureStdout, so these are literal)
	require.Contains(t, out, "✓")
	require.Contains(t, out, "✗")
	require.Contains(t, out, "!")

	// details render in the table
	require.Contains(t, out, "no access token in auth.json")
	require.Contains(t, out, "2 skill link(s) dangling")

	// hint footnotes appear only for the non-OK checks, keyed by provider/check
	require.Contains(t, out, "codex/auth: run 'codex login'")
	require.Contains(t, out, "local/skills: run 'arc skills sync'")
	require.NotContains(t, out, "ok-hint-hidden")

	// the hint footer comes after the table rows
	require.Less(t, strings.Index(out, "no access token in auth.json"), strings.Index(out, "codex/auth: run 'codex login'"))
}

func TestPrintHealth_empty(t *testing.T) {
	out := captureStdout(t, func() { PrintHealth(ai.HealthReport{}) })
	require.Contains(t, out, "no health checks ran")
}
