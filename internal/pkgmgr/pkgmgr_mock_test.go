package pkgmgr

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// darwinManager.UpdateSystem talks to the shell seam directly (no pacman/brew
// package collaborator), so its orchestration is fully deterministic to test.

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
