package mcp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/output"
	"github.com/stretchr/testify/require"
)

func renderList(t *testing.T, res ListResult, providers []Provider) string {
	t.Helper()
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	var buf bytes.Buffer
	PrintListHuman(&buf, providers, res)
	return buf.String()
}

func TestPrintListHuman_glyphsEnvAndVerdict(t *testing.T) {
	t.Setenv("SET_TOKEN", "value")
	t.Setenv("MISSING_TOKEN", "")
	providers := DefaultProviders(newTestPaths(t))[:2]
	require.Equal(t, "claude", providers[0].Name())
	require.Equal(t, "codex", providers[1].Name())
	out := renderList(t, ListResult{
		CanonicalFile: "/root/ai/mcp.json",
		Servers: []ServerEntry{
			{Name: "atlassian", Type: TypeHTTP, Enabled: true, Providers: map[string]ProviderStatus{
				"claude": {Status: StatusOK}, "codex": {Status: StatusOK},
			}},
			{Name: "homelab", Type: TypeHTTP, Enabled: true, EnvRefs: []string{"SET_TOKEN"}, Providers: map[string]ProviderStatus{
				"claude": {Status: StatusOK}, "codex": {Status: StatusDrift, Detail: "edited elsewhere"},
			}},
			{Name: "nba", Type: TypeStdio, Enabled: true, EnvRefs: []string{"MISSING_TOKEN"}, Providers: map[string]ProviderStatus{
				"claude": {Status: StatusOK}, "codex": {Status: StatusOK},
			}},
		},
		Unmanaged: []UnmanagedEntry{{Provider: "codex", Name: "openaiDeveloperDocs"}},
	}, providers)

	lines := strings.Split(out, "\n")
	require.Equal(t, "name       type   env            set  claude  codex", lines[3])
	require.Equal(t, "atlassian  http   —               —     ✓       ✓", lines[4])
	require.Equal(t, "homelab    http   SET_TOKEN       ✓     ✓       ≠", lines[5])
	require.Equal(t, "nba        stdio  MISSING_TOKEN   ⚠     ✓       ✓", lines[6])
	require.Contains(t, out, "⚠ env not set  MISSING_TOKEN")
	require.Contains(t, out, "≠ drift        codex › homelab  edited elsewhere  arc mcp sync")
	require.Contains(t, out, "⚠ 1 unmanaged  codex › openaiDeveloperDocs        arc mcp import")
	require.True(t, strings.HasSuffix(out, "⚠ 1 entry out of sync · arc mcp sync\n"))
}

func TestCheckLine_inSync(t *testing.T) {
	res := ListResult{Servers: []ServerEntry{{Name: "a", Providers: map[string]ProviderStatus{"claude": {Status: StatusOK}}}}}
	require.Equal(t, 0, res.OutOfSync())
	require.Equal(t, "✓ 1 server in sync across 1 provider", CheckLine(output.Style{Unicode: true}, 1, res))
}

func TestOutOfSync_countsOnlyFixableStates(t *testing.T) {
	res := ListResult{Servers: []ServerEntry{{Name: "a", Providers: map[string]ProviderStatus{
		"claude": {Status: StatusUnsupported}, "codex": {Status: StatusDisabled},
		"cursor": {Status: StatusMissing}, "opencode": {Status: StatusConflict},
	}}}}
	require.Equal(t, 2, res.OutOfSync())
}
