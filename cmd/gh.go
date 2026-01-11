package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "GitHub workflow management",
	Long:  `Manage GitHub workflows. Use subcommands to perform specific actions.`,
}

var ghRestartDashboardCmd = &cobra.Command{
	Use:   "restart-dashboard",
	Short: "Restart the dashboard GitHub workflow",
	Long:  `Trigger the vm_cron_restart.yml workflow in the nba_elt_dashboard repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("gh") {
			return fmt.Errorf("gh CLI is not available in PATH")
		}

		output.Info("Triggering GitHub workflow...")
		if _, err := shell.Run("gh", "workflow", "run", "vm_cron_restart.yml", "--repo", "jyablonski/nba_elt_dashboard", "--ref", "master"); err != nil {
			return fmt.Errorf("failed to trigger workflow: %w", err)
		}

		output.Success("Workflow triggered successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ghCmd)
	ghCmd.AddCommand(ghRestartDashboardCmd)
}
