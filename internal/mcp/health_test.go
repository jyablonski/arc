package mcp

import (
	"testing"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findCheck(checks []ai.HealthCheck, name string) (ai.HealthCheck, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return ai.HealthCheck{}, false
}

// No canonical file is a normal state, not a broken one, so health stays quiet.
func TestHealthChecks_SilentWithoutCanonical(t *testing.T) {
	m, _ := newTestManager(t)
	assert.Empty(t, HealthChecks(m))
}

func TestHealthChecks_OKWhenSynced(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})
	_, err := m.Sync()
	require.NoError(t, err)

	c, ok := findCheck(HealthChecks(m), "mcp")
	require.True(t, ok)
	assert.Equal(t, ai.HealthOK, c.Status)
	assert.Contains(t, c.Detail, "in sync")
}

func TestHealthChecks_WarnsOnMissing(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	c, ok := findCheck(HealthChecks(m), "mcp")
	require.True(t, ok)
	assert.Equal(t, ai.HealthWarn, c.Status)
	assert.Contains(t, c.Detail, "missing")
	assert.Contains(t, c.Hint, "arc mcp sync")
}

// The failure that actually bites: everything wired correctly, but the token
// variable is not exported, so every tool fails to authenticate at once.
func TestHealthChecks_WarnsOnUnsetEnvVar(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"homelab": {Type: TypeHTTP, URL: "http://mcp.home/mcp",
			Headers: map[string]string{"Authorization": "Bearer {env:ARC_DEFINITELY_UNSET_VAR}"}},
	})

	c, ok := findCheck(HealthChecks(m), "mcp env")
	require.True(t, ok)
	assert.Equal(t, ai.HealthWarn, c.Status)
	assert.Contains(t, c.Detail, "$ARC_DEFINITELY_UNSET_VAR")
}

func TestHealthChecks_OKWhenEnvVarSet(t *testing.T) {
	t.Setenv("ARC_TEST_MCP_TOKEN", "value")
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{
		"homelab": {Type: TypeHTTP, URL: "http://mcp.home/mcp",
			Headers: map[string]string{"Authorization": "Bearer {env:ARC_TEST_MCP_TOKEN}"}},
	})

	c, ok := findCheck(HealthChecks(m), "mcp env")
	require.True(t, ok)
	assert.Equal(t, ai.HealthOK, c.Status)
}

func TestHealthChecks_SurfacesClaudeNeedsAuthCache(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})
	writeFile(t, paths.ClaudeAuthCache,
		`{"claude.ai Gmail":{"timestamp":1784388775534,"id":"a"},"claude.ai Google Drive":{"timestamp":1,"id":"b"}}`)

	c, ok := findCheck(HealthChecks(m), "mcp auth")
	require.True(t, ok)
	assert.Equal(t, ai.HealthWarn, c.Status)
	assert.Contains(t, c.Detail, "claude.ai Gmail")
	assert.Contains(t, c.Detail, "2 MCP entries")
}

func TestHealthChecks_NoAuthCacheNoCheck(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	_, ok := findCheck(HealthChecks(m), "mcp auth")
	assert.False(t, ok)
}

func TestHealthChecks_WarnsOnConflict(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.CursorMCP, `{"mcpServers":{"shared":{"type":"stdio","command":"theirs"}}}`)
	writeCanonical(t, paths, map[string]Server{"shared": stdioServer("ours")})
	_, err := m.Sync()
	require.NoError(t, err)

	c, ok := findCheck(HealthChecks(m), "mcp")
	require.True(t, ok)
	assert.Equal(t, ai.HealthWarn, c.Status)
	assert.Contains(t, c.Detail, "conflicting")
	assert.Contains(t, c.Hint, "--force")
}
