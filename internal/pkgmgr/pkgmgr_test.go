package pkgmgr

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
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
	require.True(t, errors.Is(mgr.Install(InstallOptions{Packages: []string{"git"}}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Clean(CleanOptions{}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Installed(InstalledOptions{}), ErrUnsupportedPlatform))
	require.True(t, errors.Is(mgr.Packages(PackageOptions{}), ErrUnsupportedPlatform))
}

func TestInstall_rejectsInvalidTargetsAndConflictingSourcesBeforeProbing(t *testing.T) {
	for _, opts := range []InstallOptions{
		{},
		{Packages: []string{""}},
		{Packages: []string{"valid", "--noconfirm"}},
		{Packages: []string{"-Syu"}},
		{Packages: []string{"two packages"}},
		{Packages: []string{"package\nname"}},
		{Packages: []string{"x"}, AUR: true, Cask: true},
		{Packages: []string{"x"}, Cask: true, Formula: true},
		{Packages: []string{"x"}, AUR: true, Formula: true},
	} {
		setRunner(t, &boundary.ShellRunnerMock{})
		require.Error(t, linuxManager{}.Install(opts))
		require.Error(t, darwinManager{}.Install(opts))
	}
}
