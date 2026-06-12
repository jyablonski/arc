package pkgmgr

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

// noSudo is a runner that fails the test if any sudo call is made — used by the
// Installed/Packages tests, which must not shell out at all.
func noSudo(t *testing.T) *boundary.ShellRunnerMock {
	return &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			t.Fatalf("unexpected RunSudo: %s %v", name, args)
			return "", nil
		},
	}
}

func TestLinuxClean_pacmanUnavailable(t *testing.T) {
	m := linuxManager{pac: &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return shell.NewErrToolNotAvailable("pacman") },
	}}
	setRunner(t, noSudo(t))

	err := m.Clean(CleanOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pacman is not available")
}

func TestLinuxClean_fullCleanNoOrphans(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetOrphanedPackagesFunc:  func() ([]string, error) { return nil, nil },
	}
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) { return "", nil },
	}
	setRunner(t, mock)

	require.NoError(t, linuxManager{pac: pac}.Clean(CleanOptions{}))

	calls := mock.RunSudoCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "pacman", calls[0].Name)
	assert.Equal(t, []string{"-Sc", "--noconfirm"}, calls[0].Args)
	require.Len(t, pac.GetOrphanedPackagesCalls(), 1)
}

func TestLinuxClean_fullCleanWithOrphans(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetOrphanedPackagesFunc:  func() ([]string, error) { return []string{"orphan1", "orphan2"}, nil },
	}
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) { return "", nil },
	}
	setRunner(t, mock)

	require.NoError(t, linuxManager{pac: pac}.Clean(CleanOptions{}))

	calls := mock.RunSudoCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"-Sc", "--noconfirm"}, calls[0].Args)
	assert.Equal(t, "pacman", calls[1].Name)
	assert.Equal(t, []string{"-Rns", "--noconfirm", "orphan1", "orphan2"}, calls[1].Args)
}

func TestLinuxClean_cacheOnlySkipsOrphans(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetOrphanedPackagesFunc: func() ([]string, error) {
			t.Fatal("orphan lookup must not run in cache-only mode")
			return nil, nil
		},
	}
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) { return "", nil },
	}
	setRunner(t, mock)

	require.NoError(t, linuxManager{pac: pac}.Clean(CleanOptions{CacheOnly: true}))
	require.Len(t, mock.RunSudoCalls(), 1)
	require.Empty(t, pac.GetOrphanedPackagesCalls())
}

func TestLinuxClean_orphansOnlySkipsCache(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetOrphanedPackagesFunc:  func() ([]string, error) { return nil, nil },
	}
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			t.Fatalf("cache clean must not run in orphans-only mode: %v", args)
			return "", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxManager{pac: pac}.Clean(CleanOptions{OrphansOnly: true}))
	require.Len(t, pac.GetOrphanedPackagesCalls(), 1)
}

func TestLinuxClean_cacheCleanFails(t *testing.T) {
	pac := &pacmanOpsMock{CheckPacmanAvailableFunc: func() error { return nil }}
	setRunner(t, &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			return "", errors.New("permission denied")
		},
	})

	err := linuxManager{pac: pac}.Clean(CleanOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clean cache")
}

func TestLinuxClean_orphanRemovalFailureIsNonFatal(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetOrphanedPackagesFunc:  func() ([]string, error) { return []string{"orphan1"}, nil },
	}
	setRunner(t, &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "-Rns" {
				return "", errors.New("removal failed")
			}
			return "", nil
		},
	})

	// A failed orphan removal is only a warning; Clean still succeeds.
	require.NoError(t, linuxManager{pac: pac}.Clean(CleanOptions{}))
}

func TestLinuxInstalled_explicitList(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetExplicitlyInstalledFunc: func() ([]string, error) {
			return []string{"base 1-1", "linux 6.7-1", "vim 9.1-1"}, nil
		},
	}
	setRunner(t, noSudo(t))

	out := captureStdout(t, func() {
		require.NoError(t, linuxManager{pac: pac}.Installed(InstalledOptions{}))
	})
	assert.Contains(t, out, "base 1-1")
	assert.Contains(t, out, "vim 9.1-1")
}

func TestLinuxInstalled_foreignCount(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetForeignPackagesFunc: func() ([]string, error) {
			return []string{"yay 12.3-1", "paru 2.0-1"}, nil
		},
	}
	setRunner(t, noSudo(t))

	out := captureStdout(t, func() {
		require.NoError(t, linuxManager{pac: pac}.Installed(InstalledOptions{ForeignOnly: true, Count: true}))
	})
	assert.Contains(t, out, "2")
	require.Empty(t, pac.GetExplicitlyInstalledCalls())
}

func TestLinuxInstalled_queryFails(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc:   func() error { return nil },
		GetExplicitlyInstalledFunc: func() ([]string, error) { return nil, errors.New("pacman error") },
	}
	setRunner(t, noSudo(t))

	err := linuxManager{pac: pac}.Installed(InstalledOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get installed packages")
}

func TestLinuxPackages_success(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc:        func() error { return nil },
		GetPackageCountFunc:             func() (int, error) { return 1200, nil },
		GetExplicitlyInstalledCountFunc: func() (int, error) { return 200, nil },
		GetForeignPackageCountFunc:      func() (int, error) { return 12, nil },
		GetTotalInstalledSizeFunc:       func() (float64, error) { return 42.5, nil },
		GetCacheSizeFunc:                func() (string, error) { return "3.1 GiB", nil },
		GetOrphanedPackagesFunc:         func() ([]string, error) { return nil, nil },
		GetRecentlyInstalledCountFunc:   func(days int) (int, error) { return 5, nil },
		GetLargestPackagesFunc: func(topN int) ([]pacman.PackageInfo, error) {
			return []pacman.PackageInfo{{Name: "linux", Size: "150", Unit: "MiB"}}, nil
		},
	}
	setRunner(t, noSudo(t))

	out := captureStdout(t, func() {
		require.NoError(t, linuxManager{pac: pac}.Packages(PackageOptions{Days: 7, Top: 25, JSON: true}))
	})
	assert.Contains(t, out, "\"total_packages\": 1200")
	assert.Contains(t, out, "\"foreign_packages\": 12")
}

func TestLinuxPackages_countFails(t *testing.T) {
	pac := &pacmanOpsMock{
		CheckPacmanAvailableFunc: func() error { return nil },
		GetPackageCountFunc:      func() (int, error) { return 0, errors.New("boom") },
	}
	setRunner(t, noSudo(t))

	err := linuxManager{pac: pac}.Packages(PackageOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get package count")
}
