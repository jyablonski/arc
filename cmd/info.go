package cmd

import (
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show system information",
	Long:  `Run fastfetch to display system information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("fastfetch") {
			return shell.NewErrToolNotAvailable("fastfetch")
		}

		return shell.RunInteractive("fastfetch")
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
