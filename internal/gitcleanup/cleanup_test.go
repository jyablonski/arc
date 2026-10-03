package gitcleanup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		mockRun     func(name string, args ...string) (string, error)
		mockCmdExst func(name string) bool
		expectError bool
		errContains string
		wantToolErr bool
		wantErr     error
	}{
		{
			name: "git not available",
			mockCmdExst: func(name string) bool {
				return false
			},
			expectError: true,
			wantToolErr: true,
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
			wantErr:     arcerrs.ErrNotGitRepo,
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
			mock := &boundary.ShellRunnerMock{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExst,
			}
			setRunner(t, mock)

			err := Run()

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantToolErr {
					var toolErr *shell.ErrToolNotAvailable
					assert.True(t, errors.As(err, &toolErr))
				}
				if tt.wantErr != nil {
					assert.True(t, errors.Is(err, tt.wantErr))
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRun_reportsOnlyBranchesActuallyRemoved(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	setRunner(t, &boundary.ShellRunnerMock{
		CommandExistsFunc: func(string) bool { return true },
		RunFunc: func(name string, args ...string) (string, error) {
			switch {
			case args[0] == "rev-parse" && args[1] == "--abbrev-ref":
				return "main\n", nil
			case args[0] == "branch" && args[1] == "--merged":
				return "* main\n  feature-done\n  stuck-branch", nil
			case args[0] == "branch" && args[1] == "-d" && args[2] == "stuck-branch":
				return "", fmt.Errorf("not fully merged")
			}
			return "", nil
		},
	})

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	runErr := Run()
	require.NoError(t, w.Close())
	os.Stdout = old
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, runErr)

	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	require.True(t, strings.HasPrefix(lines[0], "arc git cleanup"))
	require.True(t, strings.HasSuffix(lines[0], "main"), "the current branch is the title meta")
	require.Equal(t, []string{
		"",
		"✓ removed feature-done",
		"⚠ stuck-branch not removed: not fully merged",
		"✓ pruned remote references",
		"",
		// A failed delete is not counted as removed, and downgrades the verdict.
		"⚠ 1 merged branch removed · remotes pruned",
	}, lines[2:])
}
