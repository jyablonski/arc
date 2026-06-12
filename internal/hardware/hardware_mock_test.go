package hardware

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linuxReporter: sudo-gated components (mobo/cpu/ram) go through RunSudo; the
// failure paths each wrap a component-specific message we assert on.

func TestLinuxReporter_moboSuccess(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunSudoFunc: func(name string, args ...string) (string, error) {
			return "Base Board Information", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxReporter{}.Show([]string{"mobo"}))

	calls := mock.RunSudoCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "dmidecode", calls[0].Name)
	assert.Equal(t, []string{"-t", "2"}, calls[0].Args)
}

func TestLinuxReporter_sudoFailures(t *testing.T) {
	tests := []struct {
		name      string
		component string
		wantMsg   string
	}{
		{"mobo", "mobo", "failed to get motherboard info"},
		{"cpu", "cpu", "failed to get CPU info"},
		{"ram", "ram", "failed to get RAM info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				RunSudoFunc: func(name string, args ...string) (string, error) {
					return "", errors.New("boom")
				},
			}
			setRunner(t, mock)

			err := linuxReporter{}.Show([]string{tt.component})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestLinuxReporter_gpuDriverFailure(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "", errors.New("not found")
		},
	}
	setRunner(t, mock)

	err := linuxReporter{}.Show([]string{"gpu-driver"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nvidia-smi failed")
}

func TestLinuxReporter_gpuFindsVGAAndQueriesIt(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			// First call: bare `lspci` listing. Second: detail query for the slot.
			if len(args) == 0 {
				return "00:02.0 VGA compatible controller: Intel Corp UHD Graphics\n00:1f.0 ISA bridge: Intel Corp", nil
			}
			return "detailed gpu info", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, linuxReporter{}.Show([]string{"gpu"}))

	calls := mock.RunCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, []string(nil), calls[0].Args)
	assert.Equal(t, "lspci", calls[1].Name)
	assert.Equal(t, []string{"-v", "-s", "00:02.0"}, calls[1].Args)
}

func TestLinuxReporter_gpuNoVGAReturnsNil(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "00:1f.0 ISA bridge: Intel Corp", nil
		},
	}
	setRunner(t, mock)

	// No VGA line -> warning only, never queries a slot, returns nil.
	require.NoError(t, linuxReporter{}.Show([]string{"gpu"}))
	require.Len(t, mock.RunCalls(), 1)
}

func TestLinuxReporter_unknownComponent(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{})

	err := linuxReporter{}.Show([]string{"bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown component")
}

func TestDarwinReporter_cpuSuccess(t *testing.T) {
	mock := &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			return "Apple M1", nil
		},
	}
	setRunner(t, mock)

	require.NoError(t, darwinReporter{}.Show([]string{"cpu"}))

	calls := mock.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "sysctl", calls[0].Name)
	assert.Equal(t, []string{"-n", "machdep.cpu.brand_string"}, calls[0].Args)
}

func TestDarwinReporter_gpuDriverUnsupported(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{})

	err := darwinReporter{}.Show([]string{"gpu-driver"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported on Linux")
}
