package notify

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/stretchr/testify/assert"
)

func TestNotifiersFromEnv(t *testing.T) {
	tests := []struct {
		name           string
		slackURL       string
		discordURL     string
		includeDiscord bool
		expectCount    int
		expectError    error
	}{
		{
			name:        "slack only (default)",
			slackURL:    "https://hooks.slack.com/test",
			expectCount: 1,
		},
		{
			name:           "slack and discord",
			slackURL:       "https://hooks.slack.com/test",
			discordURL:     "https://discord.com/api/webhooks/test",
			includeDiscord: true,
			expectCount:    2,
		},
		{
			name:        "slack not configured",
			slackURL:    "",
			expectError: arcerrs.ErrSlackWebhookNotSet,
		},
		{
			name:           "discord flag set but discord url missing",
			slackURL:       "https://hooks.slack.com/test",
			discordURL:     "",
			includeDiscord: true,
			expectError:    arcerrs.ErrDiscordWebhookNotSet,
		},
		{
			name:        "discord url set but flag not passed",
			slackURL:    "https://hooks.slack.com/test",
			discordURL:  "https://discord.com/api/webhooks/test",
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SLACK_WEBHOOK_URL", tt.slackURL)
			t.Setenv("DISCORD_WEBHOOK_URL", tt.discordURL)

			notifiers, err := NotifiersFromEnv(tt.includeDiscord)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectError))
			} else {
				assert.NoError(t, err)
				assert.Len(t, notifiers, tt.expectCount)
			}
		})
	}
}
