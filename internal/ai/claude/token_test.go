package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validCredsJSON = `{"claudeAiOauth":{"accessToken":"sk-ant-oat-test","refreshToken":"","expiresAt":null}}`

// fatalRunner fails the test if any shell call is made — used to prove a path
// resolves without shelling out.
func fatalRunner(t *testing.T) *boundary.ShellRunnerMock {
	return &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			t.Fatalf("unexpected shell call: %s %v", name, args)
			return "", nil
		},
	}
}

func writeCredsFile(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(body), 0o600))
}

func TestReadOAuthWithMeta_fromCredentialsFile(t *testing.T) {
	home := t.TempDir()
	writeCredsFile(t, home, validCredsJSON)
	// File is found first, so the Keychain branch must never run even on darwin.
	setGOOS(t, "darwin")
	setRunner(t, fatalRunner(t))

	loaded, err := readOAuthWithMeta(home)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-oat-test", loaded.AccessToken)
	assert.Equal(t, filepath.Join(home, ".claude", ".credentials.json"), loaded.CredsPath)
	assert.Equal(t, loaded.CredsPath, loaded.PersistPath)
}

func TestReadOAuthWithMeta_noTokenNonDarwin(t *testing.T) {
	home := t.TempDir() // no credentials file
	setGOOS(t, "linux")
	setRunner(t, fatalRunner(t))

	_, err := readOAuthWithMeta(home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Claude OAuth token")
}

func TestReadOAuthWithMeta_keychainJSON(t *testing.T) {
	home := t.TempDir() // no file -> falls through to Keychain
	setGOOS(t, "darwin")
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return validCredsJSON, nil
		},
	}
	setRunner(t, mock)

	loaded, err := readOAuthWithMeta(home)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-oat-test", loaded.AccessToken)
	assert.Equal(t, "macOS Keychain (Claude Code-credentials)", loaded.CredsPath)

	calls := mock.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "security", calls[0].Name)
	assert.Equal(t, []string{"find-generic-password", "-s", "Claude Code-credentials", "-w"}, calls[0].Args)
}

func TestReadOAuthWithMeta_keychainRawToken(t *testing.T) {
	home := t.TempDir()
	setGOOS(t, "darwin")
	setRunner(t, &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "sk-ant-oat-rawtoken\n", nil
		},
	})

	loaded, err := readOAuthWithMeta(home)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-oat-rawtoken", loaded.AccessToken)
	assert.Equal(t, "macOS Keychain (Claude Code-credentials)", loaded.CredsPath)
}

func TestReadOAuthWithMeta_keychainErrorReturnsNoToken(t *testing.T) {
	home := t.TempDir()
	setGOOS(t, "darwin")
	setRunner(t, &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "", assert.AnError
		},
	})

	_, err := readOAuthWithMeta(home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Claude OAuth token")
}
