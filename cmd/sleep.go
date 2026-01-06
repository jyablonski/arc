package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var sleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Suspend the system",
	Long:  `Suspend the system using systemctl suspend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("Suspending system...")
		if _, err := shell.RunSudo("systemctl", "suspend"); err != nil {
			return fmt.Errorf("failed to suspend: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sleepCmd)
}
