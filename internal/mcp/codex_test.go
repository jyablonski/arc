package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realCodexConfig mirrors the shape of an actual ~/.codex/config.toml: settings
// and project trust levels arc must never disturb, alongside MCP servers it
// owns.
const realCodexConfig = `model = "gpt-5.6-sol"
model_reasoning_effort = "high"

# my projects
[projects."/home/jacob/Documents/arc"]
trust_level = "trusted"

[projects."/home/jacob/Documents/homelab"]
trust_level = "trusted"

[plugins."github@openai-curated"]
enabled = true

[mcp_servers.openaiDeveloperDocs]
url = "https://developers.openai.com/mcp"

[mcp_servers.cube-analytics]
url = "http://localhost:8001/mcp"

[mcp_servers.homelab]
url = "http://mcp.home/mcp"
bearer_token_env_var = "HOMELAB_MCP_TOKEN"
`

func TestCodexRead(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, realCodexConfig)
	p := &CodexProvider{Path: paths.CodexConfig}

	servers, err := p.Read()
	require.NoError(t, err)
	require.Len(t, servers, 3)

	assert.Equal(t, "http://localhost:8001/mcp", servers["cube-analytics"].URL)
	assert.Equal(t, TypeHTTP, servers["cube-analytics"].EffectiveType())

	// bearer_token_env_var is Codex's only credential mechanism; it comes back
	// as the canonical Authorization header so it can cross to other tools.
	assert.Equal(t, map[string]string{"Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}"},
		servers["homelab"].Headers)
}

func TestCodexRead_StdioAndSubTableEnv(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, `[mcp_servers.inline]
command = "uvx"
args = ["pkg@latest", "--verbose"]
env = { FASTMCP_LOG_LEVEL = "ERROR", OTHER = "x" }

[mcp_servers.subtable]
command = "npx"

[mcp_servers.subtable.env]
NODE_ENV = "production"
`)
	servers, err := (&CodexProvider{Path: paths.CodexConfig}).Read()
	require.NoError(t, err)

	assert.Equal(t, "uvx", servers["inline"].Command)
	assert.Equal(t, []string{"pkg@latest", "--verbose"}, servers["inline"].Args)
	assert.Equal(t, map[string]string{"FASTMCP_LOG_LEVEL": "ERROR", "OTHER": "x"}, servers["inline"].Env)

	assert.Equal(t, "npx", servers["subtable"].Command)
	assert.Equal(t, map[string]string{"NODE_ENV": "production"}, servers["subtable"].Env)
}

func TestCodexRead_MultiLineArray(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, `[mcp_servers.multi]
command = "uvx"
args = [
  "first",
  "second",
]
`)
	servers, err := (&CodexProvider{Path: paths.CodexConfig}).Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, servers["multi"].Args)
}

// The whole reason Codex is spliced rather than re-marshalled: everything that
// is not an arc-owned MCP table has to survive byte-for-byte.
func TestCodexWrite_PreservesUnrelatedConfig(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, realCodexConfig)
	p := &CodexProvider{Path: paths.CodexConfig}

	require.NoError(t, p.Write(map[string]Server{
		"homelab": {Type: TypeHTTP, URL: "http://mcp.home/mcp",
			Headers: map[string]string{"Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}"}},
	}, []string{"homelab"}))

	out := readFile(t, paths.CodexConfig)
	assert.Contains(t, out, `model = "gpt-5.6-sol"`)
	assert.Contains(t, out, `model_reasoning_effort = "high"`)
	assert.Contains(t, out, `# my projects`)
	assert.Contains(t, out, `[projects."/home/jacob/Documents/arc"]`)
	assert.Contains(t, out, `[projects."/home/jacob/Documents/homelab"]`)
	assert.Contains(t, out, `[plugins."github@openai-curated"]`)

	// Servers arc does not own stay exactly where they were.
	assert.Contains(t, out, "[mcp_servers.openaiDeveloperDocs]")
	assert.Contains(t, out, "[mcp_servers.cube-analytics]")
	assert.Equal(t, 1, strings.Count(out, "[mcp_servers.homelab]"), "owned table rewritten once, not duplicated")
	assert.Contains(t, out, `bearer_token_env_var = "HOMELAB_MCP_TOKEN"`)
}

func TestCodexWrite_PreservesFollowingArrayOfTables(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, `[mcp_servers.owned]
command = "old"

[[hooks.PreToolUse]]
matcher = "Bash"
command = "check"
`)
	p := &CodexProvider{Path: paths.CodexConfig}

	require.NoError(t, p.Write(map[string]Server{
		"owned": {Type: TypeStdio, Command: "new"},
	}, []string{"owned"}))

	out := readFile(t, paths.CodexConfig)
	assert.Contains(t, out, "[[hooks.PreToolUse]]")
	assert.Contains(t, out, `matcher = "Bash"`)
	assert.Contains(t, out, `command = "check"`)
	assert.Contains(t, out, `command = "new"`)
}

func TestCodexWrite_RemovesOwnedServer(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, realCodexConfig)
	p := &CodexProvider{Path: paths.CodexConfig}

	require.NoError(t, p.Write(map[string]Server{}, []string{"cube-analytics"}))

	out := readFile(t, paths.CodexConfig)
	assert.NotContains(t, out, "cube-analytics")
	assert.Contains(t, out, "[mcp_servers.homelab]")
	assert.Contains(t, out, `model = "gpt-5.6-sol"`)
}

func TestCodexWrite_RoundTrips(t *testing.T) {
	paths := newTestPaths(t)
	p := &CodexProvider{Path: paths.CodexConfig}

	in := map[string]Server{
		"stdio-srv": {Type: TypeStdio, Command: "uvx", Args: []string{"pkg@latest"},
			Env: map[string]string{"LOG": "ERROR", "TOKEN": "{env:TOKEN}"}},
		"http-srv": {Type: TypeHTTP, URL: "https://example.com/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer {env:TOK}",
				"X-Tenant":      "homelab",
				"X-Trace":       "{env:TRACE_ID}",
			}},
		"dashed-name": {Type: TypeHTTP, URL: "https://example.com/dashed"},
		"off-srv":     {Type: TypeStdio, Command: "npx", Enabled: boolPtr(false)},
	}
	require.NoError(t, p.Write(in, nil))

	got, err := p.Read()
	require.NoError(t, err)
	require.Len(t, got, len(in))
	for name, want := range in {
		assert.True(t, p.Normalize(want).Equivalent(got[name]),
			"%s did not round-trip: wrote %+v, read %+v", name, want, got[name])
	}
	assert.Contains(t, readFile(t, paths.CodexConfig), "enabled = false")
}

// Codex cannot express these, so Supports must reject them rather than let
// Write emit config that silently does not work.
func TestCodexSupports_RejectsUntranslatable(t *testing.T) {
	p := &CodexProvider{Path: "/nonexistent"}

	err := p.Supports("sse-srv", Server{Type: TypeSSE, URL: "https://example.com/sse"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sse")

	err = p.Supports("hdr-srv", Server{Type: TypeHTTP, URL: "https://x",
		Headers: map[string]string{"X-Custom": "{env:VALUE}"}})
	require.NoError(t, err)

	err = p.Supports("mixed-hdr-srv", Server{Type: TypeHTTP, URL: "https://x",
		Headers: map[string]string{"X-Custom": "prefix-{env:VALUE}"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly {env:VAR}")

	require.NoError(t, p.Supports("ok-srv", Server{Type: TypeHTTP, URL: "https://x",
		Headers: map[string]string{"Authorization": "Bearer {env:TOK}"}}))
}

func TestCodexRead_MarksUnmodeledFields(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, `[mcp_servers.custom]
command = "uvx"
cwd = "/srv/custom"
`)

	servers, err := (&CodexProvider{Path: paths.CodexConfig}).Read()
	require.NoError(t, err)
	assert.True(t, servers["custom"].unmodeled)
}

func TestCodexWrite_NoCanonicalLeavesFileAlone(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CodexConfig, "model = \"gpt-5.6-sol\"\n")
	p := &CodexProvider{Path: paths.CodexConfig}

	require.NoError(t, p.Write(map[string]Server{}, nil))
	assert.Equal(t, "model = \"gpt-5.6-sol\"\n", readFile(t, paths.CodexConfig))
}

func TestTOMLKeyQuoting(t *testing.T) {
	// TOML bare keys allow dashes, so a dashed server name needs no quoting.
	assert.Equal(t, "cube-analytics", tomlKey("cube-analytics"))
	assert.Equal(t, `"has.dot"`, tomlKey("has.dot"))
}

func TestParseTableHeader(t *testing.T) {
	path, ok := parseTableHeader(`[projects."/home/x"]`)
	require.True(t, ok)
	assert.Equal(t, []string{"projects", "/home/x"}, path)

	_, ok = parseTableHeader(`[[array_of_tables]]`)
	assert.False(t, ok)

	_, ok = parseTableHeader(`key = "value"`)
	assert.False(t, ok)
}

func TestStripTOMLComment(t *testing.T) {
	assert.Equal(t, `url = "http://x" `, stripTOMLComment(`url = "http://x" # trailing`))
	assert.Equal(t, `url = "http://x#y"`, stripTOMLComment(`url = "http://x#y"`))
}
