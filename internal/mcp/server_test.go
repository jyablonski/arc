package mcp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPaths_EnvOverrides(t *testing.T) {
	t.Setenv("HOME", "/home/fake")
	t.Setenv("ARC_MCP_FILE", "/tmp/arc/mcp.json")
	t.Setenv("ARC_MCP_STATE", "/tmp/arc/state.json")
	t.Setenv("ARC_CLAUDE_JSON", "/tmp/arc/claude.json")
	t.Setenv("ARC_CODEX_CONFIG", "/tmp/arc/config.toml")
	t.Setenv("ARC_CURSOR_MCP", "/tmp/arc/cursor.json")
	t.Setenv("ARC_OPENCODE_CONFIG", "/tmp/arc/opencode.json")

	p := DefaultPaths()
	assert.Equal(t, "/tmp/arc/mcp.json", p.CanonicalFile)
	assert.Equal(t, "/tmp/arc/state.json", p.StateFile)
	assert.Equal(t, "/tmp/arc/claude.json", p.ClaudeJSON)
	assert.Equal(t, "/tmp/arc/config.toml", p.CodexConfig)
	assert.Equal(t, "/tmp/arc/cursor.json", p.CursorMCP)
	assert.Equal(t, "/tmp/arc/opencode.json", p.OpencodeConfig)
}

func TestDefaultPaths_Defaults(t *testing.T) {
	for _, key := range []string{
		"ARC_MCP_FILE", "ARC_MCP_STATE", "ARC_CLAUDE_JSON", "ARC_CODEX_CONFIG",
		"ARC_CURSOR_MCP", "ARC_OPENCODE_CONFIG", "ARC_CLAUDE_DIR", "XDG_CONFIG_HOME",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("HOME", "/home/fake")
	t.Setenv("ARC_SKILLS_ROOT", "")

	p := DefaultPaths()
	assert.Equal(t, "/home/fake/ai/mcp.json", p.CanonicalFile)
	assert.Equal(t, "/home/fake/.config/arc/mcp-state.json", p.StateFile)
	assert.Equal(t, "/home/fake/.claude.json", p.ClaudeJSON)
	assert.Equal(t, "/home/fake/.codex/config.toml", p.CodexConfig)
	assert.Equal(t, "/home/fake/.cursor/mcp.json", p.CursorMCP)
	assert.Equal(t, "/home/fake/.config/opencode/opencode.json", p.OpencodeConfig)
	assert.Equal(t, "/home/fake/.claude/mcp-needs-auth-cache.json", p.ClaudeAuthCache)
}

// The canonical file follows the skills root so the whole shared-config set
// stays in one directory.
func TestDefaultPaths_FollowsSkillsRoot(t *testing.T) {
	t.Setenv("HOME", "/home/fake")
	t.Setenv("ARC_MCP_FILE", "")
	t.Setenv("ARC_SKILLS_ROOT", "/srv/shared/skills")

	assert.Equal(t, "/srv/shared/mcp.json", DefaultPaths().CanonicalFile)
}

func TestEffectiveType(t *testing.T) {
	assert.Equal(t, TypeStdio, Server{Command: "uvx"}.EffectiveType())
	assert.Equal(t, TypeHTTP, Server{URL: "https://example.com/mcp"}.EffectiveType())
	assert.Equal(t, TypeSSE, Server{Type: TypeSSE, URL: "https://example.com/sse"}.EffectiveType())
}

func TestAppliesTo(t *testing.T) {
	all := Server{Type: TypeStdio, Command: "uvx"}
	assert.True(t, all.AppliesTo("codex"))

	restricted := Server{Type: TypeStdio, Command: "uvx", Providers: []string{"claude", "Cursor"}}
	assert.True(t, restricted.AppliesTo("claude"))
	assert.True(t, restricted.AppliesTo("cursor"), "provider match should be case-insensitive")
	assert.False(t, restricted.AppliesTo("codex"))
}

func TestEnvRefs(t *testing.T) {
	s := Server{
		Type:    TypeHTTP,
		URL:     "https://{env:HOST}/mcp",
		Headers: map[string]string{"Authorization": "Bearer {env:TOKEN}"},
	}
	assert.Equal(t, []string{"HOST", "TOKEN"}, s.EnvRefs())

	assert.Empty(t, stdioServer("uvx", "pkg").EnvRefs())
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		in      Server
		wantErr string
	}{
		{"valid stdio", "ctx7", stdioServer("uvx", "context7-mcp"), ""},
		{"valid http", "docs", httpServer("https://example.com/mcp"), ""},
		{"stdio without command", "bad", Server{Type: TypeStdio}, "needs a command"},
		{"http without url", "bad", Server{Type: TypeHTTP}, "needs a url"},
		{"stdio with url", "bad", Server{Type: TypeStdio, Command: "uvx", URL: "https://x"}, "must not set url"},
		{"http with command", "bad", Server{Type: TypeHTTP, URL: "https://x", Command: "uvx"}, "must not set command"},
		{"bad name", "has space", stdioServer("uvx"), "must match"},
		{"unknown type", "bad", Server{Type: "grpc", Command: "uvx"}, "unknown type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.server, tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// The canonical file is meant to be shareable, so a literal credential must not
// be storable in it under any spelling.
func TestValidate_RejectsInlineSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   Server
	}{
		{"literal bearer header", Server{Type: TypeHTTP, URL: "https://x", Headers: map[string]string{
			"Authorization": "Bearer abc123def456",
		}}},
		{"secretish header key", Server{Type: TypeHTTP, URL: "https://x", Headers: map[string]string{
			"X-Api-Key": "plaintext-value",
		}}},
		{"secretish env key", Server{Type: TypeStdio, Command: "uvx", Env: map[string]string{
			"GITHUB_TOKEN": "ghp_realtokenvalue123",
		}}},
		{"token prefix under innocuous key", Server{Type: TypeStdio, Command: "uvx", Env: map[string]string{
			"SETTING": "sk-abcdefghijklmnop",
		}}},
		{"env reference does not hide literal", Server{Type: TypeStdio, Command: "uvx", Env: map[string]string{
			"GITHUB_TOKEN": "ghp_realtokenvalue123{env:DUMMY}",
		}}},
		{"token in arguments", Server{Type: TypeStdio, Command: "uvx", Args: []string{
			"--token", "ghp_realtokenvalue123",
		}}},
		{"token in url", Server{Type: TypeHTTP, URL: "https://example.com/mcp?api_key=ghp_realtokenvalue123"}},
		{"password in url userinfo", Server{Type: TypeHTTP, URL: "https://user:password@example.com/mcp"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate("srv", tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "{env:VAR_NAME}")
		})
	}
}

func TestValidate_AllowsEnvReferences(t *testing.T) {
	require.NoError(t, Validate("srv", Server{
		Type:    TypeHTTP,
		URL:     "https://x/mcp",
		Headers: map[string]string{"Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}"},
	}))
	require.NoError(t, Validate("srv", Server{
		Type:    TypeStdio,
		Command: "uvx",
		Env:     map[string]string{"FASTMCP_LOG_LEVEL": "ERROR"},
	}))
	require.NoError(t, Validate("srv", Server{
		Type:    TypeStdio,
		Command: "uvx",
		Args:    []string{"--token", "{env:MCP_TOKEN}"},
	}))
}

func TestEquivalent(t *testing.T) {
	a := Server{Type: TypeStdio, Command: "uvx", Args: []string{"pkg"}, Env: map[string]string{"A": "1"}}
	b := Server{Type: TypeStdio, Command: "uvx", Args: []string{"pkg"}, Env: map[string]string{"A": "1"}}
	assert.True(t, a.Equivalent(b))

	// Providers is canonical-only bookkeeping and never reaches a provider file.
	b.Providers = []string{"claude"}
	assert.True(t, a.Equivalent(b))

	b.Args = []string{"other"}
	assert.False(t, a.Equivalent(b))

	assert.False(t, a.Equivalent(Server{Type: TypeStdio, Command: "uvx", Args: []string{"pkg"},
		Env: map[string]string{"A": "1"}, Enabled: boolPtr(false)}))
}

func TestLoadAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "mcp.json")

	loaded, err := Load(path)
	require.NoError(t, err, "a missing canonical file is not an error")
	assert.Empty(t, loaded.MCPServers)

	require.NoError(t, Save(path, File{MCPServers: map[string]Server{"a": stdioServer("uvx", "pkg")}}))

	loaded, err = Load(path)
	require.NoError(t, err)
	require.Contains(t, loaded.MCPServers, "a")
	assert.Equal(t, "uvx", loaded.MCPServers["a"].Command)
}

func TestLoad_RejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	writeFile(t, path, "not json")
	_, err := Load(path)
	require.Error(t, err)
}
