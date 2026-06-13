package pacman

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIgnoredPackages(t *testing.T) {
	t.Run("parses space- and comma-separated values across lines", func(t *testing.T) {
		conf := `# /etc/pacman.conf
[options]
#IgnorePkg = commented-out
IgnorePkg   = spotify  linux-custom
IgnorePkg = foo,bar
HoldPkg = base
`
		path := filepath.Join(t.TempDir(), "pacman.conf")
		require.NoError(t, os.WriteFile(path, []byte(conf), 0o600))
		old := pacmanConfPath
		pacmanConfPath = path
		t.Cleanup(func() { pacmanConfPath = old })

		got, err := GetIgnoredPackages()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"spotify", "linux-custom", "foo", "bar"}, got)
	})

	t.Run("missing config is not an error", func(t *testing.T) {
		old := pacmanConfPath
		pacmanConfPath = filepath.Join(t.TempDir(), "does-not-exist.conf")
		t.Cleanup(func() { pacmanConfPath = old })

		got, err := GetIgnoredPackages()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestCheckAvailable(t *testing.T) {
	t.Run("When checking pacman, it returns without panicking", func(t *testing.T) {
		err := CheckPacmanAvailable()
		// This test depends on the system - pacman might or might not be available
		if err != nil {
			var toolErr *shell.ErrToolNotAvailable
			assert.True(t, errors.As(err, &toolErr))
			assert.Equal(t, "pacman", toolErr.Tool)
		}
	})

	t.Run("When checking yay, it returns without panicking", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = CheckYayAvailable()
		})
	})
}
