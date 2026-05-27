package cmd

import (
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var sleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Suspend the system",
	Long:  `Suspend the system. Linux uses systemctl suspend; macOS uses pmset sleepnow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("Suspending system...")
		return app.System.Sleep()
	},
}

func init() {
	rootCmd.AddCommand(sleepCmd)
}
