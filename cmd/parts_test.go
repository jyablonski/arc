package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/hardware"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartsCmd_defaultShowsAllComponents(t *testing.T) {
	rep := &hardware.ReporterMock{ShowFunc: func(components []string) error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, Hardware: rep})()
	partsComponent = ""

	require.NoError(t, partsCmd.RunE(partsCmd, []string{}))

	calls := rep.ShowCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"mobo", "cpu", "gpu", "gpu-driver", "ram"}, calls[0].Components)
}

func TestPartsCmd_componentFlagNarrowsToOne(t *testing.T) {
	rep := &hardware.ReporterMock{ShowFunc: func(components []string) error { return nil }}
	defer setAppForTest(&App{Platform: platform.Linux, Hardware: rep})()
	partsComponent = "cpu"
	t.Cleanup(func() { partsComponent = "" })

	require.NoError(t, partsCmd.RunE(partsCmd, []string{}))

	calls := rep.ShowCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"cpu"}, calls[0].Components)
}

func TestPartsCmd_surfacesError(t *testing.T) {
	wantErr := errors.New("dmidecode failed")
	rep := &hardware.ReporterMock{ShowFunc: func(components []string) error { return wantErr }}
	defer setAppForTest(&App{Platform: platform.Linux, Hardware: rep})()
	partsComponent = ""

	require.ErrorIs(t, partsCmd.RunE(partsCmd, []string{}), wantErr)
}
