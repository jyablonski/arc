package setupdeps

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxInstaller_pacmanMissing(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return false },
	}
	setRunner(t, mock)

	err := linuxInstaller{}.Install()
	require.Error(t, err)
	var toolErr *shell.ErrToolNotAvailable
	require.True(t, errors.As(err, &toolErr))
	assert.Equal(t, "pacman", toolErr.Tool)
}

func TestLinuxInstaller_allInstalledRunsNothing(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		// Everything (pacman, every checkCmd, uv) reports present -> nothing to do.
		CommandExistsFunc: func(name string) bool { return true },
		RunInteractiveFunc: func(name string, args ...string) error {
			t.Fatalf("unexpected install: %s %v", name, args)
			return nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxInstaller{}.Install())
	require.Empty(t, mock.RunInteractiveCalls())
}

func TestLinuxInstaller_installsMissingPackage(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		// pacman + uv present (uv present avoids the curl installer); only
		// github-cli's checkCmd ("gh") is missing, so just that one installs.
		CommandExistsFunc: func(name string) bool { return name != "gh" },
		RunInteractiveFunc: func(name string, args ...string) error {
			return nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxInstaller{}.Install())

	calls := mock.RunInteractiveCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "sudo", calls[0].Name)
	assert.Equal(t, []string{"pacman", "-S", "--noconfirm", "github-cli"}, calls[0].Args)
}

func TestLinuxInstaller_installFailureIsNonFatal(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return name != "gh" },
		RunInteractiveFunc: func(name string, args ...string) error {
			return errors.New("pacman failed")
		},
	}
	setRunner(t, mock)

	// A failed install is only a warning; Install still returns nil.
	require.NoError(t, linuxInstaller{}.Install())
	require.Len(t, mock.RunInteractiveCalls(), 1)
}

func TestDarwinInstaller_brewMissing(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return false },
	}
	setRunner(t, mock)

	err := darwinInstaller{}.Install()
	require.Error(t, err)
	var toolErr *shell.ErrToolNotAvailable
	require.True(t, errors.As(err, &toolErr))
	assert.Equal(t, "brew", toolErr.Tool)
}

func TestDarwinInstaller_allInstalledRunsNothing(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return true },
		RunInteractiveFunc: func(name string, args ...string) error {
			t.Fatalf("unexpected install: %s %v", name, args)
			return nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, darwinInstaller{}.Install())
	require.Empty(t, mock.RunInteractiveCalls())
}

func TestDarwinInstaller_installsMissingPackage(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		// brew present; only "uv" is missing, so it brew-installs uv.
		CommandExistsFunc: func(name string) bool { return name != "uv" },
		RunInteractiveFunc: func(name string, args ...string) error {
			return nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, darwinInstaller{}.Install())

	calls := mock.RunInteractiveCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "brew", calls[0].Name)
	assert.Equal(t, []string{"install", "uv"}, calls[0].Args)
}
