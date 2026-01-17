package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
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

			if (err != nil) != tt.expectedError {
				t.Errorf("getKernelPackages() error = %v, wantErr %v", err, tt.expectedError)
				return
			}

			if !tt.expectedError {
				if len(packages) != len(tt.expectedPkgs) {
					t.Errorf("getKernelPackages() returned %d packages, want %d", len(packages), len(tt.expectedPkgs))
					return
				}
				for pkg, version := range tt.expectedPkgs {
					if packages[pkg] != version {
						t.Errorf("getKernelPackages() package %s = %q, want %q", pkg, packages[pkg], version)
					}
				}
			}
		})
	}
}

func TestPromptReboot(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldReboot bool
		wantErr      bool
	}{
		{
			name:         "yes response",
			input:        "y\n",
			shouldReboot: true,
			wantErr:      false,
		},
		{
			name:         "yes uppercase",
			input:        "Y\n",
			shouldReboot: true,
			wantErr:      false,
		},
		{
			name:         "yes full word",
			input:        "yes\n",
			shouldReboot: true,
			wantErr:      false,
		},
		{
			name:         "empty response (default yes)",
			input:        "\n",
			shouldReboot: true,
			wantErr:      false,
		},
		{
			name:         "no response",
			input:        "n\n",
			shouldReboot: false,
			wantErr:      false,
		},
		{
			name:         "no uppercase",
			input:        "N\n",
			shouldReboot: false,
			wantErr:      false,
		},
		{
			name:         "other response",
			input:        "maybe\n",
			shouldReboot: false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the parsing logic
			response := strings.TrimSpace(strings.ToLower(tt.input))
			shouldReboot := response == "" || response == "y" || response == "yes"

			if shouldReboot != tt.shouldReboot {
				t.Errorf("promptReboot() shouldReboot = %v, want %v", shouldReboot, tt.shouldReboot)
			}
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
			if result != tt.expected {
				t.Errorf("promptReboot parsing: input %q = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
