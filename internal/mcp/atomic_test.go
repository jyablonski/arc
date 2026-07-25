package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_CreatesWithRequestedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.json")
	require.NoError(t, writeFileAtomic(path, []byte("hello"), filemode.Private))

	assert.Equal(t, "hello", readFile(t, path))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// arc must never loosen permissions on a file a tool created restrictively:
// ~/.claude.json is 0600 in the wild.
func TestWriteFileAtomic_PreservesExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.json")
	writeFile(t, path, "old")
	require.NoError(t, os.Chmod(path, 0o600))

	require.NoError(t, writeFileAtomic(path, []byte("new"), filemode.File))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "0600 must not be widened to 0644")
	assert.Equal(t, "new", readFile(t, path))
}

func TestWriteFileAtomic_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.json")
	require.NoError(t, writeFileAtomic(path, []byte("a"), filemode.File))
	require.NoError(t, writeFileAtomic(path, []byte("b"), filemode.File))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "left a temp file: %s", e.Name())
	}
	assert.Len(t, entries, 1)
}

// A failed write must leave the previous contents fully intact rather than a
// truncated file: this is the whole point of writing through a rename.
func TestWriteFileAtomic_FailureLeavesTargetIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.json")
	require.NoError(t, writeFileAtomic(path, []byte("original"), filemode.File))

	// Make the directory read-only so creating the temp file fails.
	require.NoError(t, os.Chmod(filepath.Dir(path), 0o500))
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o700) })

	err := writeFileAtomic(path, []byte("replacement"), filemode.File)
	require.Error(t, err)
	assert.Equal(t, "original", readFile(t, path), "target must be untouched when the write fails")
}

// Every provider write goes through the atomic helper, so syncing must never
// widen the mode of a config file the tool itself created privately.
func TestSync_PreservesProviderFileModes(t *testing.T) {
	m, paths := newTestManager(t)
	writeFile(t, paths.ClaudeJSON, `{"numStartups":1,"mcpServers":{}}`)
	require.NoError(t, os.Chmod(paths.ClaudeJSON, 0o600))
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	_, err := m.Sync()
	require.NoError(t, err)

	info, err := os.Stat(paths.ClaudeJSON)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSync_CreatesProviderFilesPrivate(t *testing.T) {
	m, paths := newTestManager(t)
	writeCanonical(t, paths, map[string]Server{"ctx7": stdioServer("uvx", "pkg")})

	_, err := m.Sync()
	require.NoError(t, err)

	for _, path := range []string{paths.ClaudeJSON, paths.CodexConfig, paths.CursorMCP, paths.OpencodeConfig} {
		info, err := os.Stat(path)
		require.NoError(t, err, path)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"%s should be created private", filepath.Base(path))
	}

	// Canonical is the exception: it holds no credentials and is meant to be
	// committed to a dotfile repo.
	info, err := os.Stat(paths.CanonicalFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
