package pacman

import (
	"fmt"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPackageCount(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "multiple packages",
			mockOutput:    "package1 1.0.0-1\npackage2 2.0.0-1\npackage3 3.0.0-1",
			mockError:     nil,
			expectedCount: 3,
			expectedError: false,
		},
		{
			name:          "single package",
			mockOutput:    "package1 1.0.0-1",
			mockError:     nil,
			expectedCount: 1,
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "pacman error",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			count, err := GetPackageCount()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestGetExplicitlyInstalledCount(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "multiple packages",
			mockOutput:    "package1 1.0.0-1\npackage2 2.0.0-1",
			mockError:     nil,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "pacman error",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qe" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			count, err := GetExplicitlyInstalledCount()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestGetForeignPackageCount(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:          "multiple foreign packages",
			mockOutput:    "yay 12.0.0-1\npackage-query 1.0.0-1",
			mockError:     nil,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "pacman error",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qm" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			count, err := GetForeignPackageCount()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestGetExplicitlyInstalled(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedPkgs  []string
		expectedError bool
	}{
		{
			name:          "multiple packages",
			mockOutput:    "package1 1.0.0-1\npackage2 2.0.0-1",
			mockError:     nil,
			expectedPkgs:  []string{"package1 1.0.0-1", "package2 2.0.0-1"},
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedPkgs:  []string{},
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
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qe" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			pkgs, err := GetExplicitlyInstalled()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPkgs, pkgs)
		})
	}
}

func TestGetForeignPackages(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedPkgs  []string
		expectedError bool
	}{
		{
			name:          "multiple foreign packages",
			mockOutput:    "yay 12.0.0-1\npackage-query 1.0.0-1",
			mockError:     nil,
			expectedPkgs:  []string{"yay 12.0.0-1", "package-query 1.0.0-1"},
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedPkgs:  []string{},
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
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qm" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			pkgs, err := GetForeignPackages()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPkgs, pkgs)
		})
	}
}

func TestGetTotalInstalledSize(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedSize  float64
		expectedError bool
	}{
		{
			name: "single package KiB",
			mockOutput: `Name            : test-package
Installed Size  : 1024.00 KiB
`,
			mockError:     nil,
			expectedSize:  1024.0 / 1024 / 1024, // KiB to GiB
			expectedError: false,
		},
		{
			name: "single package MiB",
			mockOutput: `Name            : test-package
Installed Size  : 512.50 MiB
`,
			mockError:     nil,
			expectedSize:  512.5 / 1024, // MiB to GiB
			expectedError: false,
		},
		{
			name: "single package GiB",
			mockOutput: `Name            : test-package
Installed Size  : 2.5 GiB
`,
			mockError:     nil,
			expectedSize:  2.5,
			expectedError: false,
		},
		{
			name: "multiple packages",
			mockOutput: `Name            : package1
Installed Size  : 1024.00 KiB
Name            : package2
Installed Size  : 512.00 MiB
Name            : package3
Installed Size  : 1.0 GiB
`,
			mockError:     nil,
			expectedSize:  (1024.0/1024 + 512.0 + 1.0*1024) / 1024, // All converted to MiB then to GiB
			expectedError: false,
		},
		{
			name:          "pacman error",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedSize:  0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qi" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			size, err := GetTotalInstalledSize()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.InDelta(t, tt.expectedSize, size, 0.001)
		})
	}
}

func TestGetOrphanedPackages(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedPkgs  []string
		expectedError bool
	}{
		{
			name:          "orphaned packages",
			mockOutput:    "orphan1 1.0.0-1\norphan2 2.0.0-1",
			mockError:     nil,
			expectedPkgs:  []string{"orphan1", "orphan2"},
			expectedError: false,
		},
		{
			name:          "no orphans (exit status 1)",
			mockOutput:    "",
			mockError:     fmt.Errorf("exit status 1"),
			expectedPkgs:  []string{},
			expectedError: false,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedPkgs:  []string{},
			expectedError: false,
		},
		{
			name:          "pacman error (not exit status 1)",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedPkgs:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qdt" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			pkgs, err := GetOrphanedPackages()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPkgs, pkgs)
		})
	}
}

func TestSearchPackages(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		mockOutput    string
		mockError     error
		expectedPkgs  []string
		expectedError bool
	}{
		{
			name:          "search results",
			query:         "vim",
			mockOutput:    "extra/vim 9.0.0000-1\ncommunity/vim-plugins 1.0.0-1",
			mockError:     nil,
			expectedPkgs:  []string{"extra/vim 9.0.0000-1", "community/vim-plugins 1.0.0-1"},
			expectedError: false,
		},
		{
			name:          "empty results",
			query:         "nonexistent",
			mockOutput:    "",
			mockError:     nil,
			expectedPkgs:  []string{},
			expectedError: false,
		},
		{
			name:          "pacman error",
			query:         "vim",
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedPkgs:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 2 && args[0] == "-Ss" && args[1] == tt.query {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			pkgs, err := SearchPackages(tt.query)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPkgs, pkgs)
		})
	}
}

func TestGetCacheSize(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedSize  string
		expectedError bool
	}{
		{
			name:          "successful cache size",
			mockOutput:    "2.5G\t/var/cache/pacman/pkg",
			mockError:     nil,
			expectedSize:  "2.5G",
			expectedError: false,
		},
		{
			name:          "cache size with MiB",
			mockOutput:    "512M\t/var/cache/pacman/pkg",
			mockError:     nil,
			expectedSize:  "512M",
			expectedError: false,
		},
		{
			name:          "du error",
			mockOutput:    "",
			mockError:     fmt.Errorf("du: command not found"),
			expectedSize:  "",
			expectedError: true,
		},
		{
			name:          "empty output",
			mockOutput:    "",
			mockError:     nil,
			expectedSize:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "du" && len(args) == 2 && args[0] == "-sh" && args[1] == "/var/cache/pacman/pkg" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			size, err := GetCacheSize()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

func TestGetRecentlyInstalledCount(t *testing.T) {
	// Calculate dates relative to today for the test
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := now.AddDate(0, 0, -2).Format("2006-01-02")
	tenDaysAgo := now.AddDate(0, 0, -10).Format("2006-01-02")

	tests := []struct {
		name          string
		days          int
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "recent packages",
			days: 7,
			mockOutput: fmt.Sprintf(`[%sT10:00:00-0500] [ALPM] installed vim (9.0.0000-1)
[%sT10:00:00-0500] [ALPM] installed git (2.40.0-1)
[%sT10:00:00-0500] [ALPM] installed old-package (1.0.0-1)`, today, yesterday, tenDaysAgo),
			mockError:     nil,
			expectedCount: 2, // Only today and yesterday are within 7 days
			expectedError: false,
		},
		{
			name: "all packages within range",
			days: 30,
			mockOutput: fmt.Sprintf(`[%sT10:00:00-0500] [ALPM] installed vim (9.0.0000-1)
[%sT10:00:00-0500] [ALPM] installed git (2.40.0-1)
[%sT10:00:00-0500] [ALPM] installed old-package (1.0.0-1)`, today, yesterday, twoDaysAgo),
			mockError:     nil,
			expectedCount: 3, // All are within 30 days
			expectedError: false,
		},
		{
			name:          "no recent packages",
			days:          7,
			mockOutput:    "",
			mockError:     nil,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name:          "grep error",
			days:          7,
			mockOutput:    "",
			mockError:     fmt.Errorf("grep: command not found"),
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "grep" && len(args) >= 3 && args[0] == "-E" && args[1] == `\[ALPM\] installed` {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			count, err := GetRecentlyInstalledCount(tt.days)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestGetLargestPackages(t *testing.T) {
	tests := []struct {
		name          string
		topN          int
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "multiple packages",
			topN: 3,
			mockOutput: `Name            : package1
Installed Size  : 2.5 GiB
Name            : package2
Installed Size  : 1.0 GiB
Name            : package3
Installed Size  : 512.00 MiB
Name            : package4
Installed Size  : 256.00 MiB`,
			mockError:     nil,
			expectedCount: 3,
			expectedError: false,
		},
		{
			name: "topN larger than available",
			topN: 10,
			mockOutput: `Name            : package1
Installed Size  : 1.0 GiB
Name            : package2
Installed Size  : 512.00 MiB`,
			mockError:     nil,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "pacman error",
			topN:          5,
			mockOutput:    "",
			mockError:     fmt.Errorf("pacman: command not found"),
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "pacman" && len(args) == 1 && args[0] == "-Qi" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			pkgs, err := GetLargestPackages(tt.topN)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, pkgs, tt.expectedCount)

			// Verify packages are sorted by size (descending)
			for i := 1; i < len(pkgs); i++ {
				assert.GreaterOrEqual(t, pkgs[i-1].InstalledSize, pkgs[i].InstalledSize, "packages should be sorted by size descending")
			}
		})
	}
}
