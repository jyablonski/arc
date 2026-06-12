package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstalledCmd_threadsFlagsToManager(t *testing.T) {
	var got pkgmgr.InstalledOptions
	mgr := &pkgmgr.ManagerMock{
		InstalledFunc: func(opts pkgmgr.InstalledOptions) error { got = opts; return nil },
	}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	installedAUROnly = true
	installedCount = true
	t.Cleanup(func() { installedAUROnly = false; installedCount = false })

	require.NoError(t, installedCmd.RunE(installedCmd, []string{}))
	require.Len(t, mgr.InstalledCalls(), 1)
	assert.Equal(t, pkgmgr.InstalledOptions{ForeignOnly: true, Count: true}, got)
}

func TestInstalledCmd_darwinAUROnlyRejectedBeforeManager(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{
		InstalledFunc: func(opts pkgmgr.InstalledOptions) error { return nil },
	}
	defer setAppForTest(&App{Platform: platform.Darwin, PkgMgr: mgr})()

	installedAUROnly = true
	t.Cleanup(func() { installedAUROnly = false })

	err := installedCmd.RunE(installedCmd, []string{})
	require.ErrorIs(t, err, arcerrs.ErrAUROnlyLinuxOnly)
	require.Empty(t, mgr.InstalledCalls(), "manager must not be called when the guard rejects")
}

func TestInstalledCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("query failed")
	mgr := &pkgmgr.ManagerMock{InstalledFunc: func(opts pkgmgr.InstalledOptions) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	require.ErrorIs(t, installedCmd.RunE(installedCmd, []string{}), wantErr)
}
