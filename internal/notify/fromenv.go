package notify

import (
	"os"

	"github.com/jyablonski/arc/internal/arcerrs"
)

func NotifiersFromEnv(includeDiscord bool) ([]Notifier, error) {
	var notifiers []Notifier

	slackURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackURL == "" {
		return nil, arcerrs.ErrSlackWebhookNotSet
	}
	notifiers = append(notifiers, NewSlack(slackURL))

	if includeDiscord {
		discordURL := os.Getenv("DISCORD_WEBHOOK_URL")
		if discordURL == "" {
			return nil, arcerrs.ErrDiscordWebhookNotSet
		}
		notifiers = append(notifiers, NewDiscord(discordURL))
	}

	return notifiers, nil
}
