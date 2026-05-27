package cmd

import (
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install required packages and tools",
	Long: `Install required packages and tools needed for arc to function properly.
This includes uv, gh (GitHub CLI), fastfetch, and other system utilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Header("Setting up arc dependencies")
		if err := app.Setup.Install(); err != nil {
			return err
		}
		output.Header("Setup complete")
		output.Info("Run 'arc validate' to check if all dependencies are available")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
