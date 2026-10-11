package pkgmgr

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// darwinManager.UpdateSystem talks to the shell seam directly (no pacman/brew
// package collaborator), so its orchestration is fully deterministic to test.

func TestDarwinInstall_selectedType(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts InstallOptions
		args []string
	}{
		{"default", InstallOptions{Packages: []string{"first", "second"}}, []string{"install", "first", "second"}},
		{"cask", InstallOptions{Packages: []string{"some-app"}, Cask: true}, []string{"install", "--cask", "some-app"}},
		{"formula", InstallOptions{Packages: []string{"user/tap/tool@1"}, Formula: true}, []string{"install", "--formula", "user/tap/tool@1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				CommandExistsFunc:  func(name string) bool { return name == "brew" },
				RunInteractiveFunc: func(string, ...string) error { return nil },
			}
			setRunner(t, mock)
			require.NoError(t, darwinManager{}.Install(tt.opts))
			calls := mock.RunInteractiveCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "brew", calls[0].Name)
			assert.Equal(t, tt.args, calls[0].Args)
		})
	}
}

func TestDarwinInstall_missingBrew(t *testing.T) {
	mock := &boundary.ShellRunnerMock{CommandExistsFunc: func(string) bool { return false }}
	setRunner(t, mock)
	err := darwinManager{}.Install(InstallOptions{Packages: []string{"x"}})
	var toolErr *shell.ErrToolNotAvailable
	require.ErrorAs(t, err, &toolErr)
	require.Equal(t, "brew", toolErr.Tool)
	require.Empty(t, mock.RunInteractiveCalls())
}

func TestDarwinInstall_rejectsAURBeforeProbing(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{})
	require.ErrorIs(t, darwinManager{}.Install(InstallOptions{Packages: []string{"x"}, AUR: true}), arcerrs.ErrAURLinuxOnly)
}

func TestDarwinInstall_failureDoesNotRetryOrSwitchType(t *testing.T) {
	want := errors.New("formula not found")
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc:  func(string) bool { return true },
		RunInteractiveFunc: func(string, ...string) error { return want },
	}
	setRunner(t, mock)
	err := darwinManager{}.Install(InstallOptions{Packages: []string{"missing-package"}, Formula: true})
	require.ErrorIs(t, err, want)
	require.Len(t, mock.RunInteractiveCalls(), 1)
}

func TestDarwinUpdateSystem_brewMissing(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return false },
	}
	setRunner(t, mock)

	err := darwinManager{}.UpdateSystem(UpdateOptions{})
	require.Error(t, err)
	var toolErr *shell.ErrToolNotAvailable
	require.True(t, errors.As(err, &toolErr))
	assert.Equal(t, "brew", toolErr.Tool)
}

func TestDarwinUpdateSystem_success(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc:  func(name string) bool { return true },
		RunInteractiveFunc: func(name string, args ...string) error { return nil },
	}
	setRunner(t, mock)

	require.NoError(t, darwinManager{}.UpdateSystem(UpdateOptions{}))

	calls := mock.RunInteractiveCalls()
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"update"}, calls[0].Args)
	assert.Equal(t, []string{"upgrade"}, calls[1].Args)
	assert.Equal(t, []string{"cleanup"}, calls[2].Args)
}

func TestDarwinUpdateSystem_skipCacheOmitsCleanup(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc:  func(name string) bool { return true },
		RunInteractiveFunc: func(name string, args ...string) error { return nil },
	}
	setRunner(t, mock)

	require.NoError(t, darwinManager{}.UpdateSystem(UpdateOptions{SkipCache: true}))

	calls := mock.RunInteractiveCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"update"}, calls[0].Args)
	assert.Equal(t, []string{"upgrade"}, calls[1].Args)
}

func TestDarwinUpdateSystem_updateFails(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return true },
		RunInteractiveFunc: func(name string, args ...string) error {
			if len(args) > 0 && args[0] == "update" {
				return errors.New("network down")
			}
			return nil
		},
	}
	setRunner(t, mock)

	err := darwinManager{}.UpdateSystem(UpdateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "brew update failed")
	// Should bail after the failing update, never reaching upgrade/cleanup.
	require.Len(t, mock.RunInteractiveCalls(), 1)
}
