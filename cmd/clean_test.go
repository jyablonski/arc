package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestCleanCmd(t *testing.T) {
	tests := []struct {
		name         string
		orphansOnly  bool
		cacheOnly    bool
		mockRun      func(name string, args ...string) (string, error)
		mockCmdExist func(name string) bool
		expectError  bool
		errContains  string
	}{
		{
			name: "pacman not available",
			mockCmdExist: func(name string) bool {
				return false
			},
			expectError: true,
			errContains: "pacman is not available",
		},
		{
			name: "full clean with no orphans",
			mockCmdExist: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				// RunSudo calls Run("sudo", "pacman", ...)
				if name == "sudo" && len(args) > 0 && args[0] == "pacman" {
					if contains(args, "-Sc") {
						return "cache cleaned", nil
					}
				}
				// pacman -Qdt for orphans
				if name == "pacman" && len(args) > 0 && args[0] == "-Qdt" {
					return "", fmt.Errorf("exit status 1: ") // no orphans
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name: "full clean with orphans",
			mockCmdExist: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "sudo" && len(args) > 0 && args[0] == "pacman" {
					if contains(args, "-Sc") {
						return "cache cleaned", nil
					}
					if contains(args, "-Rns") {
						return "removed", nil
					}
				}
				if name == "pacman" && len(args) > 0 && args[0] == "-Qdt" {
					return "orphan1 1.0-1\norphan2 2.0-1", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:        "cache only mode",
			cacheOnly:   true,
			orphansOnly: false,
			mockCmdExist: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "sudo" && contains(args, "-Sc") {
					return "cache cleaned", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:        "orphans only mode",
			orphansOnly: true,
			cacheOnly:   false,
			mockCmdExist: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) > 0 && args[0] == "-Qdt" {
					return "", fmt.Errorf("exit status 1: ") // no orphans
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name: "cache clean fails",
			mockCmdExist: func(name string) bool {
				return name == "pacman"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if name == "sudo" && contains(args, "-Sc") {
					return "", fmt.Errorf("permission denied")
				}
				return "", nil
			},
			expectError: true,
			errContains: "failed to clean cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanOrphansOnly = tt.orphansOnly
			cleanCacheOnly = tt.cacheOnly
			defer func() {
				cleanOrphansOnly = false
				cleanCacheOnly = false
			}()

			mock := &shell.MockRunner{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExist,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := cleanCmd.RunE(cleanCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func contains(s []string, val string) bool {
	for _, item := range s {
		if strings.Contains(item, val) {
			return true
		}
	}
	return false
}
