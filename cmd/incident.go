package cmd

import (
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/notify"
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var (
	incidentService  string
	incidentSeverity string
	incidentDiscord  bool
)

// buildNotifiers reads webhook URLs from environment variables and returns
// configured notifiers. Slack is always required. Discord is included only
// when includeDiscord is true.
func buildNotifiers(includeDiscord bool) ([]notify.Notifier, error) {
	var notifiers []notify.Notifier

	slackURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackURL == "" {
		return nil, ErrSlackWebhookNotSet
	}
	notifiers = append(notifiers, notify.NewSlack(slackURL))

	if includeDiscord {
		discordURL := os.Getenv("DISCORD_WEBHOOK_URL")
		if discordURL == "" {
			return nil, ErrDiscordWebhookNotSet
		}
		notifiers = append(notifiers, notify.NewDiscord(discordURL))
	}

	return notifiers, nil
}

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

		notifiers, err := buildNotifiers(incidentDiscord)
		if err != nil {
			return err
		}

		incident := notify.Incident{
			Title:    title,
			Service:  incidentService,
			Severity: incidentSeverity,
		}

		output.Info(fmt.Sprintf("Sending incident alert: %s [%s] (%s)", title, incidentSeverity, incidentService))

		var sendErrors []error
		for _, n := range notifiers {
			if err := n.Send(incident); err != nil {
				output.Warning(fmt.Sprintf("Failed to send to %s: %v", n.Name(), err))
				sendErrors = append(sendErrors, err)
			} else {
				output.Success(fmt.Sprintf("Sent to %s", n.Name()))
			}
		}

		if len(sendErrors) == len(notifiers) {
			return ErrAllNotifiersFailed
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
