package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

// newTestPaths points every provider file at a temp dir so tests never read or
// write the real dotfiles.
func newTestPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		CanonicalFile:   filepath.Join(dir, "ai", "mcp.json"),
		StateFile:       filepath.Join(dir, "state", "mcp-state.json"),
		ClaudeJSON:      filepath.Join(dir, "claude.json"),
		CodexConfig:     filepath.Join(dir, "codex", "config.toml"),
		CursorMCP:       filepath.Join(dir, "cursor", "mcp.json"),
		OpencodeConfig:  filepath.Join(dir, "opencode", "opencode.json"),
		ClaudeAuthCache: filepath.Join(dir, "claude", "mcp-needs-auth-cache.json"),
	}
}

func newTestManager(t *testing.T) (*Manager, Paths) {
	t.Helper()
	paths := newTestPaths(t)
	return New(Config{Paths: paths}), paths
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), filemode.Dir))
	require.NoError(t, os.WriteFile(path, []byte(content), filemode.File))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func writeCanonical(t *testing.T, paths Paths, servers map[string]Server) {
	t.Helper()
	require.NoError(t, Save(paths.CanonicalFile, File{MCPServers: servers}))
}

func boolPtr(b bool) *bool { return &b }

// stdioServer and httpServer keep test fixtures readable.
func stdioServer(command string, args ...string) Server {
	return Server{Type: TypeStdio, Command: command, Args: args}
}

func httpServer(url string) Server {
	return Server{Type: TypeHTTP, URL: url}
}
