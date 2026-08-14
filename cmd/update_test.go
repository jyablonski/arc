package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSelfCmd_runs(t *testing.T) {
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"update", "self"})
	require.NoError(t, rootCmd.Execute())
}

func TestUpdateUvCmd_uvMissing(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return name != "uv" },
	})
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"update", "uv"})
	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestUpdateSystemCmd_threadsFlagsToManager(t *testing.T) {
	var got pkgmgr.UpdateOptions
	mgr := &pkgmgr.ManagerMock{
		UpdateSystemFunc: func(opts pkgmgr.UpdateOptions) error { got = opts; return nil },
	}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	updateNoAUR = true
	updateNoCache = true
	updateYes = true
	updateLog = true
	t.Cleanup(func() { updateNoAUR = false; updateNoCache = false; updateYes = false; updateLog = false })

	require.NoError(t, updateSystemCmd.RunE(updateSystemCmd, []string{}))
	require.Len(t, mgr.UpdateSystemCalls(), 1)
	assert.Equal(t, pkgmgr.UpdateOptions{SkipAUR: true, SkipCache: true, AssumeYes: true, Log: true}, got)
}

func TestUpdateSystemCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("update failed")
	mgr := &pkgmgr.ManagerMock{UpdateSystemFunc: func(opts pkgmgr.UpdateOptions) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	require.ErrorIs(t, updateSystemCmd.RunE(updateSystemCmd, []string{}), wantErr)
}

func TestUpdateSystemCmd_darwinNoAURRejectedBeforeManager(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{
		UpdateSystemFunc: func(opts pkgmgr.UpdateOptions) error { return nil },
	}
	defer setAppForTest(&App{Platform: platform.Darwin, PkgMgr: mgr})()

	flag := updateSystemCmd.Flags().Lookup("no-aur")
	oldChanged := flag.Changed
	oldNoAUR := updateNoAUR
	flag.Changed = true
	updateNoAUR = true
	t.Cleanup(func() {
		flag.Changed = oldChanged
		updateNoAUR = oldNoAUR
	})

	err := updateSystemCmd.RunE(updateSystemCmd, []string{})
	require.ErrorIs(t, err, arcerrs.ErrNoAURLinuxOnly)
	require.Empty(t, mgr.UpdateSystemCalls(), "manager must not be called when the guard rejects")
}

func TestUpdateSystemCmd_darwinYesRejectedBeforeManager(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{
		UpdateSystemFunc: func(opts pkgmgr.UpdateOptions) error { return nil },
	}
	defer setAppForTest(&App{Platform: platform.Darwin, PkgMgr: mgr})()

	flag := updateSystemCmd.Flags().Lookup("yes")
	oldChanged := flag.Changed
	oldYes := updateYes
	flag.Changed = true
	updateYes = true
	t.Cleanup(func() {
		flag.Changed = oldChanged
		updateYes = oldYes
	})

	err := updateSystemCmd.RunE(updateSystemCmd, []string{})
	require.ErrorIs(t, err, arcerrs.ErrAssumeYesLinuxOnly)
	require.Empty(t, mgr.UpdateSystemCalls(), "manager must not be called when the guard rejects")
}

func TestUpdateSystemCmd_darwinLogRejectedBeforeManager(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{
		UpdateSystemFunc: func(opts pkgmgr.UpdateOptions) error { return nil },
	}
	defer setAppForTest(&App{Platform: platform.Darwin, PkgMgr: mgr})()

	flag := updateSystemCmd.Flags().Lookup("log")
	oldChanged := flag.Changed
	oldLog := updateLog
	flag.Changed = true
	updateLog = true
	t.Cleanup(func() {
		flag.Changed = oldChanged
		updateLog = oldLog
	})

	err := updateSystemCmd.RunE(updateSystemCmd, []string{})
	require.ErrorIs(t, err, arcerrs.ErrUpdateLogLinuxOnly)
	require.Empty(t, mgr.UpdateSystemCalls(), "manager must not be called when the guard rejects")
}
