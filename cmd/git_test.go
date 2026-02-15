package cmd

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestFilterMergedBranches(t *testing.T) {
	tests := []struct {
		name          string
		mergedOutput  string
		currentBranch string
		expected      []string
	}{
		{
			name: "filters current branch and main/master",
			mergedOutput: `* main
  feature-one
  feature-two
  master`,
			currentBranch: "main",
			expected:      []string{"feature-one", "feature-two"},
		},
		{
			name: "current branch is feature",
			mergedOutput: `  main
* feature-active
  feature-done
  old-branch`,
			currentBranch: "feature-active",
			expected:      []string{"feature-done", "old-branch"},
		},
		{
			name:          "only main and master",
			mergedOutput:  "  main\n  master",
			currentBranch: "main",
			expected:      []string{},
		},
		{
			name:          "empty output",
			mergedOutput:  "",
			currentBranch: "main",
			expected:      []string{},
		},
		{
			name:          "only current branch",
			mergedOutput:  "* develop",
			currentBranch: "develop",
			expected:      []string{},
		},
		{
			name: "whitespace handling",
			mergedOutput: `  main
  feature-branch
  another-branch`,
			currentBranch: "main",
			expected:      []string{"feature-branch", "another-branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterMergedBranches(tt.mergedOutput, tt.currentBranch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitCmd(t *testing.T) {
	tests := []struct {
		name        string
		mockRun     func(name string, args ...string) (string, error)
		mockCmdExst func(name string) bool
		expectError bool
		errContains string
	}{
		{
			name: "git not available",
			mockCmdExst: func(name string) bool {
				return false
			},
			expectError: true,
			errContains: "git is not available",
		},
		{
			name: "not in git repo",
			mockCmdExst: func(name string) bool {
				return name == "git"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "rev-parse" && args[1] == "--git-dir" {
					return "", fmt.Errorf("fatal: not a git repository")
				}
				return "", nil
			},
			expectError: true,
			errContains: "not in a git repository",
		},
		{
			name: "successful cleanup with merged branches",
			mockCmdExst: func(name string) bool {
				return name == "git"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if len(args) == 0 {
					return "", nil
				}
				switch args[0] {
				case "rev-parse":
					if len(args) > 1 && args[1] == "--git-dir" {
						return ".git", nil
					}
					if len(args) > 1 && args[1] == "--abbrev-ref" {
						return "main", nil
					}
				case "branch":
					if len(args) > 1 && args[1] == "--merged" {
						return "* main\n  feature-done\n  old-branch", nil
					}
					if len(args) > 1 && args[1] == "-d" {
						return "", nil
					}
				case "remote":
					return "", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name: "no merged branches to clean",
			mockCmdExst: func(name string) bool {
				return name == "git"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if len(args) == 0 {
					return "", nil
				}
				switch args[0] {
				case "rev-parse":
					if len(args) > 1 && args[1] == "--git-dir" {
						return ".git", nil
					}
					if len(args) > 1 && args[1] == "--abbrev-ref" {
						return "main", nil
					}
				case "branch":
					return "* main", nil
				case "remote":
					return "", nil
				}
				return "", nil
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExst,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := gitCmd.RunE(gitCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
