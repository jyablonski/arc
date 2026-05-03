package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/notify"
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var (
	incidentService  string
	incidentSeverity string
	incidentDiscord  bool
)

var incidentCmd = &cobra.Command{
	Use:   "incident [title]",
	Short: "Trigger an incident alert to Slack (and optionally Discord)",
	Long: `Send an incident alert to Slack, and optionally to Discord as well.

Set webhook URLs via environment variables:
  SLACK_WEBHOOK_URL    Slack incoming webhook URL (required)
  DISCORD_WEBHOOK_URL  Discord webhook URL (required when --discord is used)

By default, alerts are sent to Slack only. Use --discord to also send to Discord.

Examples:
  arc incident "database is down" --service api --severity p1
  arc incident "high latency on checkout" --service payments --severity p2
  arc incident "cert expiring soon" --service infra --severity p3 --discord`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		notifiers, err := notify.NotifiersFromEnv(incidentDiscord)
		if err != nil {
			return err
		}

		inc := notify.Incident{
			Title:    title,
			Service:  incidentService,
			Severity: incidentSeverity,
		}

		output.Info(fmt.Sprintf("Sending incident alert: %s [%s] (%s)", title, incidentSeverity, incidentService))

		var sendErrors []error
		for _, n := range notifiers {
			if err := n.Send(inc); err != nil {
				output.Warning(fmt.Sprintf("Failed to send to %s: %v", n.Name(), err))
				sendErrors = append(sendErrors, err)
			} else {
				output.Success(fmt.Sprintf("Sent to %s", n.Name()))
			}
		}

		if len(sendErrors) == len(notifiers) {
			return arcerrs.ErrAllNotifiersFailed
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(incidentCmd)
	incidentCmd.Flags().StringVar(&incidentService, "service", "unknown", "Affected service name")
	incidentCmd.Flags().StringVar(&incidentSeverity, "severity", "p3", "Severity level (p1, p2, p3)")
	incidentCmd.Flags().BoolVar(&incidentDiscord, "discord", false, "Also send to Discord")
}
