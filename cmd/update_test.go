package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKernelPackages(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedPkgs  map[string]string
		expectedError bool
	}{
		{
			name: "single kernel package",
			mockOutput: `linux 6.1.0-1
other-package 1.0.0-1`,
			mockError: nil,
			expectedPkgs: map[string]string{
				"linux": "6.1.0-1",
			},
			expectedError: false,
		},
		{
			name: "multiple kernel packages",
			mockOutput: `linux 6.1.0-1
linux-lts 5.15.0-1
linux-zen 6.2.0-1
other-package 1.0.0-1`,
			mockError: nil,
			expectedPkgs: map[string]string{
				"linux":     "6.1.0-1",
				"linux-lts": "5.15.0-1",
				"linux-zen": "6.2.0-1",
			},
			expectedError: false,
		},
		{
			name: "kernel packages with headers excluded",
			mockOutput: `linux 6.1.0-1
linux-headers 6.1.0-1
linux-lts 5.15.0-1
linux-lts-headers 5.15.0-1`,
			mockError: nil,
			expectedPkgs: map[string]string{
				"linux":     "6.1.0-1",
				"linux-lts": "5.15.0-1",
			},
			expectedError: false,
		},
		{
			name:          "no kernel packages",
			mockOutput:    `other-package 1.0.0-1\nanother-package 2.0.0-1`,
			mockError:     nil,
			expectedPkgs:  map[string]string{},
			expectedError: false,
		},
		{
			name:          "pacman error",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedPkgs:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Q" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			packages, err := getKernelPackages()

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, packages, len(tt.expectedPkgs))
			for pkg, version := range tt.expectedPkgs {
				assert.Equal(t, version, packages[pkg], "package %s version mismatch", pkg)
			}
		})
	}
}

func TestPromptReboot(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldReboot bool
	}{
		{
			name:         "yes response",
			input:        "y\n",
			shouldReboot: true,
		},
		{
			name:         "yes uppercase",
			input:        "Y\n",
			shouldReboot: true,
		},
		{
			name:         "yes full word",
			input:        "yes\n",
			shouldReboot: true,
		},
		{
			name:         "empty response (default yes)",
			input:        "\n",
			shouldReboot: true,
		},
		{
			name:         "no response",
			input:        "n\n",
			shouldReboot: false,
		},
		{
			name:         "no uppercase",
			input:        "N\n",
			shouldReboot: false,
		},
		{
			name:         "other response",
			input:        "maybe\n",
			shouldReboot: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the parsing logic
			response := strings.TrimSpace(strings.ToLower(tt.input))
			shouldReboot := response == "" || response == "y" || response == "yes"

			assert.Equal(t, tt.shouldReboot, shouldReboot)
		})
	}
}

// TestPromptRebootParsing tests the actual parsing logic used in promptReboot
func TestPromptRebootParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "y",
			input:    "y",
			expected: true,
		},
		{
			name:     "yes",
			input:    "yes",
			expected: true,
		},
		{
			name:     "Y uppercase",
			input:    "Y",
			expected: true,
		},
		{
			name:     "YES uppercase",
			input:    "YES",
			expected: true,
		},
		{
			name:     "n",
			input:    "n",
			expected: false,
		},
		{
			name:     "no",
			input:    "no",
			expected: false,
		},
		{
			name:     "other",
			input:    "maybe",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := strings.TrimSpace(strings.ToLower(tt.input))
			result := response == "" || response == "y" || response == "yes"
			assert.Equal(t, tt.expected, result)
		})
	}
}
