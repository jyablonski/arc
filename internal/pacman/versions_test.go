package pacman

import (
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/require"
)

func TestGetInstalledPackageVersions(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "pacman", name)
			require.Equal(t, []string{"-Q"}, args)
			return "linux 1:6.12.1-1\nfoo 2.0-3\n", nil
		},
	})

	versions, err := GetInstalledPackageVersions()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"linux": "1:6.12.1-1", "foo": "2.0-3"}, versions)
}
