package pacman

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInstalledKernelPackages(t *testing.T) {
	tests := []struct {
		name         string
		mockOutput   string
		expectedPkgs map[string]string
	}{
		{
			name: "single kernel package",
			mockOutput: `linux 6.1.0-1
other-package 1.0.0-1`,
			expectedPkgs: map[string]string{"linux": "6.1.0-1"},
		},
		{
			name: "multiple kernel packages",
			mockOutput: `linux 6.1.0-1
linux-lts 5.15.0-1
linux-zen 6.2.0-1
other-package 1.0.0-1`,
			expectedPkgs: map[string]string{
				"linux": "6.1.0-1", "linux-lts": "5.15.0-1", "linux-zen": "6.2.0-1",
			},
		},
		{
			name: "kernel packages with headers excluded",
			mockOutput: `linux 6.1.0-1
linux-headers 6.1.0-1
linux-lts 5.15.0-1
linux-lts-headers 5.15.0-1`,
			expectedPkgs: map[string]string{"linux": "6.1.0-1", "linux-lts": "5.15.0-1"},
		},
		{
			name:         "no kernel packages",
			mockOutput:   "other-package 1.0.0-1\nanother-package 2.0.0-1",
			expectedPkgs: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packages := ParseInstalledKernelPackages(tt.mockOutput)
			assert.Len(t, packages, len(tt.expectedPkgs))
			for pkg, version := range tt.expectedPkgs {
				assert.Equal(t, version, packages[pkg], "package %s version mismatch", pkg)
			}
		})
	}
}

func TestInstalledKernelVersionsPacmanErrors(t *testing.T) {
	mock := &shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			if name == "pacman" && len(args) == 1 && args[0] == "-Q" {
				return "", fmt.Errorf("pacman: command not found")
			}
			return "", fmt.Errorf("unexpected command: %s %v", name, args)
		},
	}
	shell.SetMockRunner(mock)
	defer shell.ClearMockRunner()

	_, err := InstalledKernelVersions()
	require.Error(t, err)
}
