package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanCmd_threadsFlagsToManager(t *testing.T) {
	tests := []struct {
		name        string
		orphansOnly bool
		cacheOnly   bool
	}{
		{"default", false, false},
		{"orphans only", true, false},
		{"cache only", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got pkgmgr.CleanOptions
			mgr := &pkgmgr.ManagerMock{
				CleanFunc: func(opts pkgmgr.CleanOptions) error { got = opts; return nil },
			}
			defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

			cleanOrphansOnly = tt.orphansOnly
			cleanCacheOnly = tt.cacheOnly
			t.Cleanup(func() { cleanOrphansOnly = false; cleanCacheOnly = false })

			require.NoError(t, cleanCmd.RunE(cleanCmd, []string{}))
			require.Len(t, mgr.CleanCalls(), 1)
			assert.Equal(t, pkgmgr.CleanOptions{OrphansOnly: tt.orphansOnly, CacheOnly: tt.cacheOnly}, got)
		})
	}
}

func TestCleanCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("clean failed")
	mgr := &pkgmgr.ManagerMock{CleanFunc: func(opts pkgmgr.CleanOptions) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr})()

	require.ErrorIs(t, cleanCmd.RunE(cleanCmd, []string{}), wantErr)
}
