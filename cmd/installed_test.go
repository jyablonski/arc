package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	return buf.String()
}

func TestInstalledCmd(t *testing.T) {
	tests := []struct {
		name        string
		aurOnly     bool
		count       bool
		mockRun     func(name string, args ...string) (string, error)
		mockCmdExst func(name string) bool
		expectError bool
		errContains string
		checkOutput func(t *testing.T, output string)
	}{
		{
			name:    "list explicitly installed packages",
			aurOnly: false,
			count:   false,
			mockCmdExst: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) > 0 && args[0] == "-Qe" {
					return "base 1-1\nlinux 6.7-1\nvim 9.1-1", nil
				}
				return "", nil
			},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "base 1-1")
				assert.Contains(t, output, "linux 6.7-1")
				assert.Contains(t, output, "vim 9.1-1")
			},
		},
		{
			name:    "count explicitly installed packages",
			aurOnly: false,
			count:   true,
			mockCmdExst: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) > 0 && args[0] == "-Qe" {
					return "base 1-1\nlinux 6.7-1\nvim 9.1-1", nil
				}
				return "", nil
			},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "3")
			},
		},
		{
			name:    "list AUR only packages",
			aurOnly: true,
			count:   false,
			mockCmdExst: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) > 0 && args[0] == "-Qm" {
					return "yay 12.3-1\nparu 2.0-1", nil
				}
				return "", nil
			},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "yay 12.3-1")
				assert.Contains(t, output, "paru 2.0-1")
			},
		},
		{
			name:    "count AUR only packages",
			aurOnly: true,
			count:   true,
			mockCmdExst: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) > 0 && args[0] == "-Qm" {
					return "yay 12.3-1\nparu 2.0-1", nil
				}
				return "", nil
			},
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "2")
			},
		},
		{
			name:    "pacman not available",
			aurOnly: false,
			count:   false,
			mockCmdExst: func(name string) bool {
				return false
			},
			expectError: true,
			errContains: "pacman is not available",
		},
		{
			name:    "pacman query fails",
			aurOnly: false,
			count:   false,
			mockCmdExst: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("pacman error")
			},
			expectError: true,
			errContains: "failed to get installed packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set package-level flags for the test
			installedAUROnly = tt.aurOnly
			installedCount = tt.count
			defer func() {
				installedAUROnly = false
				installedCount = false
			}()

			mock := &shell.MockRunner{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExst,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			if tt.expectError {
				err := installedCmd.RunE(installedCmd, []string{})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				output := captureStdout(t, func() {
					err := installedCmd.RunE(installedCmd, []string{})
					assert.NoError(t, err)
				})
				if tt.checkOutput != nil {
					tt.checkOutput(t, output)
				}
			}
		})
	}
}

func TestInstalledCmd_darwinListsFormulae(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	installedAUROnly = false
	installedCount = false
	t.Cleanup(func() {
		installedAUROnly = false
		installedCount = false
	})

	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "brew" },
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "brew", name)
			require.Equal(t, []string{"list", "--formula"}, args)
			return "git\nuv\n", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	output := captureStdout(t, func() {
		require.NoError(t, installedCmd.RunE(installedCmd, []string{}))
	})
	require.Contains(t, output, "git")
	require.Contains(t, output, "uv")
}

func TestInstalledCmd_darwinAUROnlyUnsupported(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	installedAUROnly = true
	t.Cleanup(func() { installedAUROnly = false })

	err := installedCmd.RunE(installedCmd, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--aur-only is only supported on Linux")
}
