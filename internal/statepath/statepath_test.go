package statepath

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArcDir_respectsXDGStateHome(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)

	dir, err := ArcDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "arc"), dir)
}
