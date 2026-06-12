package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagesCmd_threadsFlagsToManager(t *testing.T) {
	var got pkgmgr.PackageOptions
	mgr := &pkgmgr.ManagerMock{
		PackagesFunc: func(opts pkgmgr.PackageOptions) error { got = opts; return nil },
	}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	packagesDays = 14
	packagesTop = 10
	packagesJSON = true
	t.Cleanup(func() { packagesDays = 0; packagesTop = 0; packagesJSON = false })

	require.NoError(t, packagesCmd.RunE(packagesCmd, []string{}))
	require.Len(t, mgr.PackagesCalls(), 1)
	assert.Equal(t, pkgmgr.PackageOptions{Days: 14, Top: 10, JSON: true}, got)
}

func TestPackagesCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("packages failed")
	mgr := &pkgmgr.ManagerMock{PackagesFunc: func(opts pkgmgr.PackageOptions) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	require.ErrorIs(t, packagesCmd.RunE(packagesCmd, []string{}), wantErr)
}
