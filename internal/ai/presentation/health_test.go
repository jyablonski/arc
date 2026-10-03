package presentation

import (
	"bytes"
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

func renderHealth(t *testing.T, report ai.HealthReport) string {
	t.Helper()
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	var buf bytes.Buffer
	PrintHealth(&buf, report)
	return buf.String()
}

func TestPrintHealth_allOK_ordersProvidersAndHidesHints(t *testing.T) {
	// Deliberately out of order; the printer must sort claude → codex → cursor → local.
	report := ai.HealthReport{Checks: []ai.HealthCheck{
		{Category: "config", Name: "skills", Status: ai.HealthOK, Detail: "6 skills linked"},
		{Category: "auth", Name: "cursor", Status: ai.HealthOK, Detail: "cursor token ok"},
		{Category: "auth", Name: "codex", Status: ai.HealthOK, Detail: "codex token ok", Hint: "hint-should-not-render"},
		{Category: "auth", Name: "claude", Status: ai.HealthOK, Detail: "claude token ok"},
	}}

	out := renderHealth(t, report)

	require.True(t, strings.HasPrefix(out, "arc ai health"))
	require.Contains(t, out, "provider")
	// row ordering: claude before codex before cursor before local
	require.Less(t, strings.Index(out, "claude token ok"), strings.Index(out, "codex token ok"))
	require.Less(t, strings.Index(out, "codex token ok"), strings.Index(out, "cursor token ok"))
	require.Less(t, strings.Index(out, "cursor token ok"), strings.Index(out, "6 skills linked"))
	// no failures → no hint footer, even though an OK check carries a hint
	require.NotContains(t, out, "hint-should-not-render")
	require.NotContains(t, out, "✗")
	require.NotContains(t, out, "⚠")
	require.Contains(t, out, "✓ all 4 checks passed")
}

func TestPrintHealth_brokenChecks_showGlyphsDetailsAndHints(t *testing.T) {
	report := ai.HealthReport{Checks: []ai.HealthCheck{
		{Category: "auth", Name: "claude", Status: ai.HealthOK, Detail: "token valid for 5h", Hint: "ok-hint-hidden"},
		{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: "no access token in auth.json", Hint: "run 'codex login'"},
		{Category: "config", Name: "skills", Status: ai.HealthWarn, Detail: "2 skill link(s) dangling", Hint: "run 'arc skills sync'"},
	}}

	out := renderHealth(t, report)

	require.Contains(t, out, "✓  claude    auth    token valid for 5h")
	require.Contains(t, out, "✗  codex     auth    no access token in auth.json")
	require.Contains(t, out, "⚠  local     skills  2 skill link(s) dangling")

	// details render in the table
	require.Contains(t, out, "no access token in auth.json")
	require.Contains(t, out, "2 skill link(s) dangling")

	// hint footnotes appear only for the non-OK checks, keyed by provider/check
	require.Contains(t, out, "✗ codex/auth    run 'codex login'")
	require.Contains(t, out, "⚠ local/skills  run 'arc skills sync'")
	require.NotContains(t, out, "ok-hint-hidden")

	// the hint footer comes after the table rows
	require.Less(t, strings.Index(out, "no access token in auth.json"), strings.Index(out, "✗ codex/auth"))
	require.Contains(t, out, "✗ 1 failed · 1 warning · 3 checks")
}

func TestPrintHealth_empty(t *testing.T) {
	out := renderHealth(t, ai.HealthReport{})
	require.Contains(t, out, "no health checks ran")
}
