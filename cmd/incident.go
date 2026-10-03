package cmd

import (
	"fmt"
	"os"

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

		style := output.StyleFor(os.Stdout)
		output.Title("arc incident", incidentSeverity+style.Sep()+incidentService)
		output.Info(title)

		sent := 0
		for _, n := range notifiers {
			if err := n.Send(inc); err != nil {
				output.Error(fmt.Sprintf("%s not sent: %v", n.Name(), err))
				continue
			}
			sent++
			output.Success("sent to " + n.Name())
		}

		verdict := fmt.Sprintf("alert sent to %d of %s", sent, output.Count(len(notifiers), "channel", "channels"))
		switch sent {
		case len(notifiers):
			output.Summary(output.GlyphOK, verdict)
		case 0:
			output.Summary(output.GlyphFail, verdict)
			return arcerrs.ErrAllNotifiersFailed
		default:
			output.Summary(output.GlyphWarn, verdict)
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
