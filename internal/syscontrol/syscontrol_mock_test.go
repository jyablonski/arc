package syscontrol

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxController_sleepSuccess(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			return "", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxController{}.Sleep())

	calls := mock.RunSudoCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "systemctl", calls[0].Name)
	assert.Equal(t, []string{"suspend"}, calls[0].Args)
}

func TestLinuxController_sleepFailure(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			return "", errors.New("boom")
		},
	}
	setRunner(t, mock)

	err := linuxController{}.Sleep()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to suspend")
}

func TestDarwinController_sleepSuccess(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, darwinController{}.Sleep())

	calls := mock.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "pmset", calls[0].Name)
	assert.Equal(t, []string{"sleepnow"}, calls[0].Args)
}

func TestDarwinController_sleepFailure(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "", errors.New("boom")
		},
	}
	setRunner(t, mock)

	err := darwinController{}.Sleep()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to suspend")
}
