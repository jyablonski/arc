package gitcleanup

import (
	"fmt"
	"strings"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
)

func Run() error {
	if !shell.CommandExists("git") {
		return shell.NewErrToolNotAvailable("git")
	}

	if _, err := shell.Run("git", "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("%w: %w", arcerrs.ErrNotGitRepo, err)
	}

	output.Header("Cleaning up Git repository")

	currentBranch, err := shell.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch = strings.TrimSpace(currentBranch)

	mergedOutput, err := shell.Run("git", "branch", "--merged")
	if err != nil {
		return fmt.Errorf("failed to get merged branches: %w", err)
	}

	branchesToDelete := filterMergedBranches(mergedOutput, currentBranch)

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

	output.Info("Pruning remote references...")
	if _, err := shell.Run("git", "remote", "prune", "origin"); err != nil {
		output.Warning(fmt.Sprintf("Failed to prune remotes: %v", err))
	} else {
		output.Success("Pruned remote references")
	}

	return nil
}

func filterMergedBranches(mergedOutput, currentBranch string) []string {
	lines := strings.Split(strings.TrimSpace(mergedOutput), "\n")
	branchesToDelete := make([]string, 0)

	for _, line := range lines {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if branch == "" || branch == currentBranch || branch == "main" || branch == "master" {
			continue
		}
		branchesToDelete = append(branchesToDelete, branch)
	}

	return branchesToDelete
}
