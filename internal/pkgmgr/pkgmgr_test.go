package pkgmgr

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	require.IsType(t, linuxManager{}, New(platform.Linux))
	require.IsType(t, darwinManager{}, New(platform.Darwin))
	require.IsType(t, unsupportedManager{}, New(platform.Unknown))
}

func TestUnsupportedManager(t *testing.T) {
	mgr := New(platform.Unknown)

	require.True(t, errors.Is(mgr.UpdateSystem(UpdateOptions{}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Clean(CleanOptions{}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Installed(InstalledOptions{}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Packages(PackageOptions{}), ErrUnsupportedPlatform))
}
