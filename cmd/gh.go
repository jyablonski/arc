package cmd

import (
	"github.com/jyablonski/arc/internal/extracmd"
	"github.com/jyablonski/arc/internal/ghworkflow"
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
	Long:  `Trigger the vm_cron_restart.yml workflow in the nba_elt_dashboard repository and wait for completion.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := extracmd.EnsureAvailable(cmd); err != nil {
			return err
		}
		return ghworkflow.RestartDashboard()
	},
}

func init() {
	rootCmd.AddCommand(ghCmd)
	ghCmd.AddCommand(ghRestartDashboardCmd)
	extracmd.RegisterHiddenUnlessEnabled(ghCmd, ghRestartDashboardCmd)
}
