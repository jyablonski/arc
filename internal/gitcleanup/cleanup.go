package gitcleanup

import (
	"fmt"
	"strings"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
)

func Run() error {
	if !run.CommandExists("git") {
		return shell.NewErrToolNotAvailable("git")
	}

	if _, err := run.Run("git", "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("%w: %w", arcerrs.ErrNotGitRepo, err)
	}

	currentBranch, err := run.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch = strings.TrimSpace(currentBranch)

	mergedOutput, err := run.Run("git", "branch", "--merged")
	if err != nil {
		return fmt.Errorf("failed to get merged branches: %w", err)
	}

	output.Title("arc git cleanup", currentBranch)
	glyph := output.GlyphOK
	removed := 0
	for _, branch := range filterMergedBranches(mergedOutput, currentBranch) {
		if _, err := run.Run("git", "branch", "-d", branch); err != nil {
			glyph = output.GlyphWarn
			output.Warning(fmt.Sprintf("%s not removed: %v", branch, err))
			continue
		}
		removed++
		output.Success("removed " + branch)
	}
	if removed == 0 && glyph == output.GlyphOK {
		output.Info("no merged branches to remove")
	}

	pruned := "remotes pruned"
	if _, err := run.Run("git", "remote", "prune", "origin"); err != nil {
		glyph = output.GlyphWarn
		pruned = "remote prune failed"
		output.Warning(fmt.Sprintf("remote references not pruned: %v", err))
	} else {
		output.Success("pruned remote references")
	}

	output.Summary(glyph, output.Count(removed, "merged branch", "merged branches")+" removed", pruned)
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
