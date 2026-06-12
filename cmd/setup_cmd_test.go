package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/setupdeps"
	"github.com/stretchr/testify/require"
)

func TestSetupCmd_callsInstall(t *testing.T) {
	inst := &setupdeps.InstallerMock{InstallFunc: func() error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, Setup: inst})()

	require.NoError(t, setupCmd.RunE(setupCmd, []string{}))
	require.Len(t, inst.InstallCalls(), 1)
}

func TestSetupCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("install failed")
	inst := &setupdeps.InstallerMock{InstallFunc: func() error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, Setup: inst})()

	require.ErrorIs(t, setupCmd.RunE(setupCmd, []string{}), wantErr)
}
