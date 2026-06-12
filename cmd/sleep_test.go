package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/syscontrol"
	"github.com/stretchr/testify/require"
)

func TestSleepCmd_callsControllerSleep(t *testing.T) {
	ctrl := &syscontrol.ControllerMock{SleepFunc: func() error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, System: ctrl})()

	require.NoError(t, sleepCmd.RunE(sleepCmd, []string{}))
	require.Len(t, ctrl.SleepCalls(), 1)
}

func TestSleepCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("suspend failed")
	ctrl := &syscontrol.ControllerMock{SleepFunc: func() error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, System: ctrl})()

	require.ErrorIs(t, sleepCmd.RunE(sleepCmd, []string{}), wantErr)
}
