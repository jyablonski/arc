package cmd

import (
	"fmt"
	"strings"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git cleanup",
	Short: "Clean up Git repositories",
	Long: `Remove merged branches and prune remote references.
This should be run from within a Git repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("git") {
			return fmt.Errorf("git is not available in PATH")
		}

		// Check if we're in a git repo
		if _, err := shell.Run("git", "rev-parse", "--git-dir"); err != nil {
			return fmt.Errorf("not in a git repository: %w", err)
		}

		output.Header("Cleaning up Git repository")

		// Get current branch
		currentBranch, err := shell.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		currentBranch = strings.TrimSpace(currentBranch)

		// Get merged branches (excluding current branch and main/master)
		mergedOutput, err := shell.Run("git", "branch", "--merged")
		if err != nil {
			return fmt.Errorf("failed to get merged branches: %w", err)
		}

		lines := strings.Split(strings.TrimSpace(mergedOutput), "\n")
		branchesToDelete := []string{}

		for _, line := range lines {
			branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
			if branch == "" || branch == currentBranch || branch == "main" || branch == "master" {
				continue
			}
			branchesToDelete = append(branchesToDelete, branch)
		}

		if len(branchesToDelete) > 0 {
			output.Info(fmt.Sprintf("Removing %d merged branches...", len(branchesToDelete)))
			for _, branch := range branchesToDelete {
				if _, err := shell.Run("git", "branch", "-d", branch); err != nil {
					output.Warning(fmt.Sprintf("Failed to delete branch %s: %v", branch, err))
				}
			}
			output.Success(fmt.Sprintf("Removed %d merged branches", len(branchesToDelete)))
		} else {
			output.Info("No merged branches to remove")
		}

		// Prune remotes
		output.Info("Pruning remote references...")
		if _, err := shell.Run("git", "remote", "prune", "origin"); err != nil {
			output.Warning(fmt.Sprintf("Failed to prune remotes: %v", err))
		} else {
			output.Success("Pruned remote references")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(gitCmd)
}
