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

func renderSync(t *testing.T, res SyncResult, dryRun bool) string {
	t.Helper()
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("COLUMNS", "")
	var buf bytes.Buffer
	PrintSyncHuman(output.NewStream(&buf), res, dryRun)
	return buf.String()
}

func TestPrintSyncHuman_gridAndVerdict(t *testing.T) {
	out := renderSync(t, SyncResult{Providers: []ProviderSyncResult{
		{Provider: "claude", Path: "/etc/claude.json", Written: 12, Removed: 1},
		{Provider: "codex", Path: "/etc/codex/config.toml", Written: 3},
	}}, false)
	require.Equal(t, strings.Join([]string{
		"provider  written  removed  conflicts  unsupported  path",
		"claude         12        1          0            0  /etc/claude.json",
		"codex           3        0          0            0  /etc/codex/config.toml",
		"",
		"✓ 15 written · 1 removed",
		"",
	}, "\n"), out)
}

func TestPrintSyncHuman_conflictsAndFailuresSetTheVerdict(t *testing.T) {
	conflict := renderSync(t, SyncResult{Providers: []ProviderSyncResult{
		{Provider: "claude", Path: "/etc/claude.json", Written: 1, Conflicts: []string{"ctx7"},
			Unsupported: map[string]string{"sse-only": "no SSE transport"}},
	}}, false)
	require.Contains(t, conflict, "\n\n⚠ claude/ctx7: configured by hand and differs; left unchanged\n⚠ claude/sse-only: skipped, no SSE transport\n\n")
	require.True(t, strings.HasSuffix(conflict, "⚠ 1 conflict · 1 written · arc mcp sync --force\n"))

	failed := renderSync(t, SyncResult{Providers: []ProviderSyncResult{
		{Provider: "claude", Path: "/etc/claude.json", Error: "permission denied", Conflicts: []string{"ctx7"}},
	}}, false)
	// The error is a whole line under the grid, never a truncated cell.
	require.Contains(t, failed, "\n\n✗ claude: permission denied\n⚠ claude/ctx7: ")
	// A provider failure outranks a conflict.
	require.True(t, strings.HasSuffix(failed, "✗ 1 provider failed\n"))
}

func TestPrintSyncHuman_dryRunAndNoOp(t *testing.T) {
	dry := renderSync(t, SyncResult{Providers: []ProviderSyncResult{
		{Provider: "claude", Path: "/etc/claude.json", Written: 2, Removed: 1},
	}}, true)
	// A dry run must not claim it wrote anything.
	require.True(t, strings.HasPrefix(dry, "provider  to write  to remove  conflicts"))
	require.True(t, strings.HasSuffix(dry, "✓ 2 to write · 1 to remove\n"))
	require.NotContains(t, dry, "written")

	noop := renderSync(t, SyncResult{Providers: []ProviderSyncResult{{Provider: "claude", Path: "/etc/claude.json"}}}, false)
	require.True(t, strings.HasSuffix(noop, "✓ already in sync\n"))
}

func TestPrintImportHuman(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	render := func(res ImportResult, dryRun bool) string {
		var buf bytes.Buffer
		PrintImportHuman(output.NewStream(&buf), res, dryRun)
		return buf.String()
	}
	added := ImportResult{CanonicalFile: "/etc/ai/mcp.json", Added: []ImportedServer{{Name: "docs", Provider: "codex"}}}

	require.Equal(t, "· nothing new to import into /etc/ai/mcp.json\n", render(ImportResult{CanonicalFile: "/etc/ai/mcp.json"}, false))
	require.Equal(t, "✓ 1 imported into /etc/ai/mcp.json · arc mcp sync\n", render(added, false))
	// A dry run must not claim the import happened or suggest syncing it.
	require.Equal(t, "✓ 1 would be imported into /etc/ai/mcp.json\n", render(added, true))

	added.Conflicts = []ImportedServer{{Name: "ctx7", Provider: "claude", Reason: "differs from canonical"}}
	added.Rejected = []ImportedServer{{Name: "tok", Provider: "cursor", Reason: "literal credential"}}
	require.Equal(t, strings.Join([]string{
		"⚠ ctx7 (claude): differs from canonical",
		"✗ tok (cursor): literal credential",
		"",
		"✗ 1 imported into /etc/ai/mcp.json · 1 conflict · 1 rejected · arc mcp sync",
		"",
	}, "\n"), render(added, false))
}

// The manager reports its steps to the stream it was given, so a caller whose
// stdout carries JSON can send them elsewhere; the verdict printed on that
// same stream is set off from them and reflects any warning among them.
func TestSync_reportsStepsToTheConfiguredStream(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("COLUMNS", "")
	paths := newTestPaths(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	var log bytes.Buffer
	st := output.NewStream(&log)
	m := New(Config{Paths: paths, Providers: DefaultProviders(paths)[:1], DryRun: true, Log: st})
	res, err := m.Sync()
	require.NoError(t, err)
	PrintSyncHuman(st, res, true)

	lines := strings.Split(log.String(), "\n")
	require.Equal(t, "· would add claude/ctx7", lines[0])
	require.Empty(t, lines[1], "the grid is set off from the steps above it")
	require.True(t, strings.HasPrefix(lines[2], "provider  to write"))
	require.True(t, strings.HasSuffix(log.String(), "\n\n✓ 1 to write\n"))
}
