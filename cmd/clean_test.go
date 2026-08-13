package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/sysupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanCmd_threadsFlagsToManager(t *testing.T) {
	tests := []struct {
		name        string
		orphansOnly bool
		cacheOnly   bool
		logsOnly    bool
		wantManager bool
		wantLogs    bool
	}{
		{"default", false, false, false, true, true},
		{"orphans only", true, false, false, true, false},
		{"cache only", false, true, false, true, false},
		{"logs only", false, false, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got pkgmgr.CleanOptions
			var cleanedLogs bool
			mgr := &pkgmgr.ManagerMock{
				CleanFunc: func(opts pkgmgr.CleanOptions) error { got = opts; return nil },
			}
			defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()
			oldCleanUpdateLogs := cleanUpdateLogs
			cleanUpdateLogs = func() (sysupdate.LogCleanupResult, error) {
				cleanedLogs = true
				return sysupdate.LogCleanupResult{}, nil
			}
			t.Cleanup(func() { cleanUpdateLogs = oldCleanUpdateLogs })

			cleanOrphansOnly = tt.orphansOnly
			cleanCacheOnly = tt.cacheOnly
			cleanLogsOnly = tt.logsOnly
			t.Cleanup(resetCleanFlags)

			require.NoError(t, cleanCmd.RunE(cleanCmd, []string{}))
			if tt.wantManager {
				require.Len(t, mgr.CleanCalls(), 1)
				assert.Equal(t, pkgmgr.CleanOptions{OrphansOnly: tt.orphansOnly, CacheOnly: tt.cacheOnly}, got)
			} else {
				require.Empty(t, mgr.CleanCalls())
			}
			assert.Equal(t, tt.wantLogs, cleanedLogs)
		})
	}
}

func TestCleanCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("clean failed")
	mgr := &pkgmgr.ManagerMock{CleanFunc: func(opts pkgmgr.CleanOptions) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()
	oldCleanUpdateLogs := cleanUpdateLogs
	var cleanedLogs bool
	cleanUpdateLogs = func() (sysupdate.LogCleanupResult, error) {
		cleanedLogs = true
		return sysupdate.LogCleanupResult{}, nil
	}
	t.Cleanup(func() { cleanUpdateLogs = oldCleanUpdateLogs })
	t.Cleanup(resetCleanFlags)

	require.ErrorIs(t, cleanCmd.RunE(cleanCmd, []string{}), wantErr)
	require.False(t, cleanedLogs)
}

func TestCleanCmd_rejectsMultipleModes(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{CleanFunc: func(pkgmgr.CleanOptions) error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()
	oldCleanUpdateLogs := cleanUpdateLogs
	cleanUpdateLogs = func() (sysupdate.LogCleanupResult, error) {
		t.Fatal("log cleanup must not run with conflicting modes")
		return sysupdate.LogCleanupResult{}, nil
	}
	t.Cleanup(func() { cleanUpdateLogs = oldCleanUpdateLogs })
	cleanCacheOnly = true
	cleanLogsOnly = true
	t.Cleanup(resetCleanFlags)

	err := cleanCmd.RunE(cleanCmd, []string{})
	require.ErrorContains(t, err, "only one of")
	require.Empty(t, mgr.CleanCalls())
}

func TestCleanCmd_surfacesLogCleanupError(t *testing.T) {
	mgr := &pkgmgr.ManagerMock{CleanFunc: func(pkgmgr.CleanOptions) error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()
	wantErr := errors.New("log cleanup failed")
	oldCleanUpdateLogs := cleanUpdateLogs
	cleanUpdateLogs = func() (sysupdate.LogCleanupResult, error) {
		return sysupdate.LogCleanupResult{}, wantErr
	}
	t.Cleanup(func() { cleanUpdateLogs = oldCleanUpdateLogs })
	cleanLogsOnly = true
	t.Cleanup(resetCleanFlags)

	err := cleanCmd.RunE(cleanCmd, []string{})
	require.ErrorIs(t, err, wantErr)
	require.Empty(t, mgr.CleanCalls())
}

func TestLogCleanupMessage(t *testing.T) {
	require.Equal(t, "Removed 1 update log (1.5 KiB)", logCleanupMessage(sysupdate.LogCleanupResult{Files: 1, Bytes: 1536}))
	require.Equal(t, "Removed 2 update logs (2.0 MiB)", logCleanupMessage(sysupdate.LogCleanupResult{Files: 2, Bytes: 2 * 1024 * 1024}))
}

func resetCleanFlags() {
	cleanOrphansOnly = false
	cleanCacheOnly = false
	cleanLogsOnly = false
}
