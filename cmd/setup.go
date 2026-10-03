package cmd

import (
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install required packages and tools",
	Long: `Install required packages and tools needed for arc to function properly.
This includes uv, gh (GitHub CLI), fastfetch, and other system utilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Title("arc setup", app.Platform.String())
		if err := app.Setup.Install(); err != nil {
			return err
		}
		output.Summary(output.GlyphOK, "setup complete", output.StyleFor(os.Stdout).Faint("arc validate"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
