package cmd

import (
	"github.com/jyablonski/arc/internal/gitcleanup"
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git cleanup",
	Short: "Clean up Git repositories",
	Long: `Remove merged branches and prune remote references.
This should be run from within a Git repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return gitcleanup.Run()
	},
}

func init() {
	rootCmd.AddCommand(gitCmd)
}
