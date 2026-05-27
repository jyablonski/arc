package cmd

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartsCmd(t *testing.T) {
	defer setAppForTest(newApp(platform.Linux))()

	tests := []struct {
		name        string
		component   string
		mockRun     func(name string, args ...string) (string, error)
		expectError bool
		errContains string
	}{
		{
			name:      "gpu-driver component success",
			component: "gpu-driver",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "nvidia-smi" {
					return "NVIDIA-SMI 545.29.06   Driver Version: 545.29.06", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:      "gpu-driver component fails",
			component: "gpu-driver",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "nvidia-smi" {
					return "", fmt.Errorf("nvidia-smi not found")
				}
				return "", nil
			},
			expectError: true,
			errContains: "nvidia-smi failed",
		},
		{
			name:      "gpu component with VGA device",
			component: "gpu",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "lspci" && len(args) == 0 {
					return "01:00.0 VGA compatible controller: NVIDIA Corporation GA106", nil
				}
				if name == "lspci" && len(args) > 0 && args[0] == "-v" {
					return "GPU detailed info here", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:      "gpu component lspci fails",
			component: "gpu",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "lspci" && len(args) == 0 {
					return "", fmt.Errorf("lspci not found")
				}
				return "", nil
			},
			expectError: true,
			errContains: "failed to get GPU PCI info",
		},
		{
			name:      "mobo component uses sudo",
			component: "mobo",
			mockRun: func(name string, args ...string) (string, error) {
				// RunSudo calls Run("sudo", "dmidecode", "-t", "2")
				if name == "sudo" && len(args) > 0 && args[0] == "dmidecode" {
					return "Manufacturer: ASUSTeK\nProduct Name: ROG STRIX", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:      "cpu component success",
			component: "cpu",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "sudo" && len(args) > 0 && args[0] == "dmidecode" {
					return "Processor: AMD Ryzen 9 7950X", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:      "ram component success",
			component: "ram",
			mockRun: func(name string, args ...string) (string, error) {
				if name == "sudo" && len(args) > 0 && args[0] == "lshw" {
					return "Size: 32GiB", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:      "invalid component",
			component: "invalid",
			mockRun: func(name string, args ...string) (string, error) {
				return "", nil
			},
			expectError: true,
			errContains: "unknown component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partsComponent = tt.component
			defer func() {
				partsComponent = ""
			}()

			mock := &shell.MockRunner{
				RunFunc: tt.mockRun,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := partsCmd.RunE(partsCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPartsCmd_darwinCPU(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	partsComponent = "cpu"
	t.Cleanup(func() { partsComponent = "" })

	shell.SetMockRunner(&shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "sysctl", name)
			require.Equal(t, []string{"-n", "machdep.cpu.brand_string"}, args)
			return "Apple M3 Pro", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, partsCmd.RunE(partsCmd, []string{}))
}

func TestPartsCmd_darwinGPUDriverUnsupported(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	partsComponent = "gpu-driver"
	t.Cleanup(func() { partsComponent = "" })

	err := partsCmd.RunE(partsCmd, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "gpu-driver is only supported on Linux")
}
