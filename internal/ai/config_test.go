package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestReadConfig_subscriptions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path, err := ConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), filemode.Dir))
	require.NoError(t, os.WriteFile(path, []byte(`{
		"subscriptions": {
			"Claude": 200,
			"codex": 20,
			"cursor": 0
		}
	}`), 0o600))

	cfg, ok, err := ReadConfig()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, map[string]float64{"claude": 200, "codex": 20}, cfg.Subscriptions)
	require.True(t, cfg.HasSubscriptions())
}
