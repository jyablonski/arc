package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync_WritesToEveryProvider(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"ctx7": stdioServer("uvx", "context7-mcp"),
	})

	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 0, res.Conflicts())
	assert.Equal(t, 0, res.Failures())
	require.Len(t, res.Providers, 4)

	for _, p := range m.Providers() {
		got, err := p.Read()
		require.NoError(t, err)
		assert.Contains(t, got, "ctx7", "%s should have the server", p.Name())
	}
}

func TestSync_IsIdempotent(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	_, err := m.Sync()
	require.NoError(t, err)
	before := readFile(t, paths.CodexConfig)

	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, before, readFile(t, paths.CodexConfig), "a second sync must not churn the file")

	// Everything is already in place, so nothing reports as drift.
	list, err := m.List()
	require.NoError(t, err)
	for _, s := range list.Servers {
		for name, ps := range s.Providers {
			assert.Equal(t, StatusOK, ps.Status, "%s/%s", name, s.Name)
		}
	}
	assert.Equal(t, 0, res.Conflicts())
}

// Removing a server from canonical must sweep it out of the providers arc wrote
// it to — that is the entire point of tracking ownership.
func TestSync_RemovesServerDroppedFromCanonical(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"keep": stdioServer("uvx", "keep"),
		"drop": stdioServer("uvx", "drop"),
	})
	_, err := m.Sync()
	require.NoError(t, err)

	writeCanonical(t, paths, map[string]Server{"keep": stdioServer("uvx", "keep")})
	res, err := m.Sync()
	require.NoError(t, err)

	for _, p := range res.Providers {
		assert.Equal(t, 1, p.Removed, "%s should report one removal", p.Provider)
	}
	for _, p := range m.Providers() {
		got, err := p.Read()
		require.NoError(t, err)
		assert.NotContains(t, got, "drop", "%s", p.Name())
		assert.Contains(t, got, "keep", "%s", p.Name())
	}
}

// A server someone configured by hand, with a different definition, is a
// conflict: sync reports it and changes nothing.
func TestSync_ReportsConflictAndLeavesFileAlone(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"shared":{"type":"stdio","command":"their-version"}}}`)
	writeCanonical(t, paths, map[string]Server{"shared": stdioServer("our-version")})

	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 1, res.Conflicts())

	got, err := (&CursorProvider{Path: paths.CursorMCP}).Read()
	require.NoError(t, err)
	assert.Equal(t, "their-version", got["shared"].Command, "conflicting entry must not be overwritten")

	// The other providers still get the server; one conflict does not block them.
	claude, err := (&ClaudeProvider{Path: paths.ClaudeJSON}).Read()
	require.NoError(t, err)
	assert.Equal(t, "our-version", claude["shared"].Command)
}

func TestSync_DoesNotOverwriteUnmodeledProviderFields(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{
  "mcpServers": {
    "shared": {
      "type": "http",
      "url": "https://example.com/mcp",
      "oauth": {"clientId": "keep-me"}
    }
  }
}`)
	writeCanonical(t, paths, map[string]Server{
		"shared": httpServer("https://example.com/mcp"),
	})

	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 1, res.Conflicts())
	assert.Contains(t, readFile(t, paths.CursorMCP), `"clientId": "keep-me"`)

	res, err = m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 1, res.Conflicts(), "the entry must remain unmanaged on later syncs")
	assert.Contains(t, readFile(t, paths.CursorMCP), `"clientId": "keep-me"`)
}

func TestSync_DisabledServerDoesNotClaimUnmanagedEntry(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{
  "mcpServers": {
    "off": {"type": "stdio", "command": "hand-managed"}
  }
}`)
	writeCanonical(t, paths, map[string]Server{
		"off": {Type: TypeStdio, Command: "canonical", Enabled: boolPtr(false)},
	})

	for range 2 {
		res, err := m.Sync()
		require.NoError(t, err)
		assert.Equal(t, 1, res.Conflicts())
		assert.Contains(t, readFile(t, paths.CursorMCP), "hand-managed")
	}

	state, err := LoadState(paths.StateFile)
	require.NoError(t, err)
	assert.False(t, state.Owns("cursor", "off"))
}

func TestSync_DisablingManagedServerRemovesAndReleasesOmittedProviderEntry(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"toggle": stdioServer("uvx"),
	})
	_, err := m.Sync()
	require.NoError(t, err)

	writeCanonical(t, paths, map[string]Server{
		"toggle": {Type: TypeStdio, Command: "uvx", Enabled: boolPtr(false)},
	})
	_, err = m.Sync()
	require.NoError(t, err)

	cursor, err := (&CursorProvider{Path: paths.CursorMCP}).Read()
	require.NoError(t, err)
	assert.NotContains(t, cursor, "toggle")
	state, err := LoadState(paths.StateFile)
	require.NoError(t, err)
	assert.False(t, state.Owns("cursor", "toggle"))
}

func TestSync_ForceOverwritesConflict(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"shared":{"type":"stdio","command":"their-version"}}}`)
	writeCanonical(t, paths, map[string]Server{"shared": stdioServer("our-version")})

	m := New(Config{Paths: paths, Force: true})
	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 0, res.Conflicts())

	got, err := (&CursorProvider{Path: paths.CursorMCP}).Read()
	require.NoError(t, err)
	assert.Equal(t, "our-version", got["shared"].Command)
}

// An identical hand-configured server is adopted silently — otherwise the very
// first sync after an import would report conflicts everywhere.
func TestSync_AdoptsIdenticalExistingServer(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CodexConfig, "[mcp_servers.homelab]\nurl = \"http://mcp.home/mcp\"\n")
	writeCanonical(t, paths, map[string]Server{"homelab": httpServer("http://mcp.home/mcp")})

	res, err := m.Sync()
	require.NoError(t, err)
	assert.Equal(t, 0, res.Conflicts())
}

func TestSync_SkipsUnsupportedAndExcluded(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"sse-srv":     {Type: TypeSSE, URL: "https://example.com/sse"},
		"claude-only": {Type: TypeStdio, Command: "uvx", Providers: []string{"claude"}},
	})

	res, err := m.Sync()
	require.NoError(t, err)

	byName := map[string]ProviderSyncResult{}
	for _, p := range res.Providers {
		byName[p.Provider] = p
	}
	assert.Contains(t, byName["codex"].Unsupported, "sse-srv", "codex has no sse transport")
	assert.NotContains(t, byName["claude"].Unsupported, "sse-srv")
	assert.Contains(t, byName["codex"].Excluded, "claude-only")

	codex, err := (&CodexProvider{Path: paths.CodexConfig}).Read()
	require.NoError(t, err)
	assert.NotContains(t, codex, "sse-srv")
	assert.NotContains(t, codex, "claude-only")

	claude, err := (&ClaudeProvider{Path: paths.ClaudeJSON}).Read()
	require.NoError(t, err)
	assert.Contains(t, claude, "sse-srv")
	assert.Contains(t, claude, "claude-only")
}

func TestSync_DryRunWritesNothing(t *testing.T) {
	paths := newTestPaths(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	m := New(Config{Paths: paths, DryRun: true})
	_, err := m.Sync()
	require.NoError(t, err)

	assert.NoFileExists(t, paths.CodexConfig)
	assert.NoFileExists(t, paths.CursorMCP)
	assert.NoFileExists(t, paths.StateFile)
}

func TestSync_RefusesCanonicalWithInlineSecret(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CanonicalFile, `{"mcpServers":{"bad":{"type":"http","url":"https://x","headers":{"Authorization":"Bearer literal-token-here"}}}}`)

	_, err := m.Sync()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arc mcp validate")
	assert.NoFileExists(t, paths.CursorMCP, "nothing is written when canonical is invalid")
}

func TestList_ReportsStatusesAndUnmanaged(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"theirs":{"type":"stdio","command":"x"}}}`)
	writeCanonical(t, paths, map[string]Server{
		"ours": stdioServer("uvx", "pkg"),
		"off":  {Type: TypeStdio, Command: "uvx", Enabled: boolPtr(false)},
	})

	res, err := m.List()
	require.NoError(t, err)
	require.Len(t, res.Servers, 2)

	byName := map[string]ServerEntry{}
	for _, s := range res.Servers {
		byName[s.Name] = s
	}
	assert.Equal(t, StatusMissing, byName["ours"].Providers["claude"].Status)
	assert.Equal(t, StatusDisabled, byName["off"].Providers["claude"].Status,
		"claude omits disabled servers, so absent is correct")

	require.Len(t, res.Unmanaged, 1)
	assert.Equal(t, "theirs", res.Unmanaged[0].Name)
	assert.Equal(t, "cursor", res.Unmanaged[0].Provider)
}

func TestList_DriftAfterExternalEdit(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})
	_, err := m.Sync()
	require.NoError(t, err)

	// Same name, edited outside arc: arc owns it, so this is drift, not conflict.
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"ctx7":{"type":"stdio","command":"edited"}}}`)

	res, err := m.List()
	require.NoError(t, err)
	assert.Equal(t, StatusDrift, res.Servers[0].Providers["cursor"].Status)
}

func TestImport_SeedsCanonicalFromProviders(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CodexConfig, realCodexConfig)

	res, err := m.Import()
	require.NoError(t, err)
	assert.Len(t, res.Added, 3)

	f, err := Load(paths.CanonicalFile)
	require.NoError(t, err)
	assert.Contains(t, f.MCPServers, "homelab")
	assert.Contains(t, f.MCPServers, "cube-analytics")
	assert.Equal(t, map[string]string{"Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}"},
		f.MCPServers["homelab"].Headers)
}

func TestImport_SkipsIdenticalAndFlagsConflicts(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"same":      httpServer("http://mcp.home/mcp"),
		"different": httpServer("http://canonical/mcp"),
	})
	writeFile(t, paths.CodexConfig, `[mcp_servers.same]
url = "http://mcp.home/mcp"

[mcp_servers.different]
url = "http://provider/mcp"
`)

	res, err := m.Import()
	require.NoError(t, err)
	assert.Empty(t, res.Added)
	require.Len(t, res.Skipped, 1)
	assert.Equal(t, "same", res.Skipped[0].Name)
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, "different", res.Conflicts[0].Name)

	f, err := Load(paths.CanonicalFile)
	require.NoError(t, err)
	assert.Equal(t, "http://canonical/mcp", f.MCPServers["different"].URL, "import never overwrites canonical")
}

// Importing a server carrying a literal token would copy a secret into a file
// meant to be shared, so it is refused.
func TestImport_RejectsInlineSecret(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"leaky":{"type":"http","url":"https://x","headers":{"Authorization":"Bearer ghp_realtoken12345"}}}}`)

	res, err := m.Import()
	require.NoError(t, err)
	assert.Empty(t, res.Added)
	require.Len(t, res.Rejected, 1)
	assert.Equal(t, "leaky", res.Rejected[0].Name)

	f, err := Load(paths.CanonicalFile)
	require.NoError(t, err)
	assert.NotContains(t, f.MCPServers, "leaky")
}

func TestImport_RejectsUnmodeledProviderFields(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.OpencodeConfig, `{
  "mcp": {
    "custom": {
      "type": "remote",
      "url": "https://example.com/mcp",
      "timeout": 15000
    }
  }
}`)

	res, err := m.Import()
	require.NoError(t, err)
	assert.Empty(t, res.Added)
	require.Len(t, res.Rejected, 1)
	assert.Contains(t, res.Rejected[0].Reason, "cannot preserve")
}

func TestAddAndRemove(t *testing.T) {
	m, paths := newTestManager(t)

	_, err := m.Add("ctx7", stdioServer("uvx", "context7-mcp"), false)
	require.NoError(t, err)

	f, err := Load(paths.CanonicalFile)
	require.NoError(t, err)
	assert.Contains(t, f.MCPServers, "ctx7")
	claude, err := (&ClaudeProvider{Path: paths.ClaudeJSON}).Read()
	require.NoError(t, err)
	assert.Contains(t, claude, "ctx7", "add syncs immediately")

	_, err = m.Add("ctx7", stdioServer("uvx", "other"), false)
	require.Error(t, err, "adding a duplicate without --force must fail")

	_, err = m.Remove("ctx7")
	require.NoError(t, err)

	f, err = Load(paths.CanonicalFile)
	require.NoError(t, err)
	assert.NotContains(t, f.MCPServers, "ctx7")
	claude, err = (&ClaudeProvider{Path: paths.ClaudeJSON}).Read()
	require.NoError(t, err)
	assert.NotContains(t, claude, "ctx7", "remove sweeps the providers")
}

func TestRemove_UnknownServer(t *testing.T) {
	m, _ := newTestManager(t)
	_, err := m.Remove("nope")
	require.Error(t, err)
}

func TestValidate_ReportsFatalAndWarnings(t *testing.T) {
	m, paths := newTestManager(t)
	t.Setenv("ARC_TEST_MCP_TOKEN", "set")
	writeFile(t, paths.CanonicalFile, `{"mcpServers":{
	  "sse-srv": {"type":"sse","url":"https://example.com/sse"},
	  "ok-srv": {"type":"http","url":"https://x","headers":{"Authorization":"Bearer {env:ARC_TEST_MCP_TOKEN}"}}
	}}`)

	issues, err := m.Validate()
	require.NoError(t, err)

	var fatal int
	var codexWarning bool
	for _, i := range issues {
		if i.Fatal {
			fatal++
		}
		if i.Provider == "codex" && i.Server == "sse-srv" {
			codexWarning = true
		}
	}
	assert.Zero(t, fatal, "an sse server is valid; it just cannot reach codex")
	assert.True(t, codexWarning, "codex should warn about the sse transport")
}

func TestValidate_FlagsUnsetEnvVar(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"srv": {Type: TypeHTTP, URL: "https://x",
			Headers: map[string]string{"Authorization": "Bearer {env:ARC_DEFINITELY_UNSET_VAR}"}},
	})

	issues, err := m.Validate()
	require.NoError(t, err)
	require.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Error, "ARC_DEFINITELY_UNSET_VAR")
	assert.False(t, issues[0].Fatal)
}

func TestFilterProviders(t *testing.T) {
	providers := DefaultProviders(newTestPaths(t))

	got, err := FilterProviders(providers, []string{"codex", "claude"})
	require.NoError(t, err)
	assert.Equal(t, []string{"codex", "claude"}, ProviderNames(got))

	all, err := FilterProviders(providers, nil)
	require.NoError(t, err)
	assert.Len(t, all, 4)

	_, err = FilterProviders(providers, []string{"nope"})
	require.Error(t, err)
}

func TestState_CorruptFileFallsBackToOwningNothing(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.StateFile, "{{{ not json")

	st, err := LoadState(paths.StateFile)
	require.NoError(t, err, "a corrupt state file must not wedge sync")
	assert.False(t, st.Owns("claude", "anything"))
}
