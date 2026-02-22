package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildNotifiers(t *testing.T) {
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
			expectError: ErrSlackWebhookNotSet,
		},
		{
			name:           "discord flag set but discord url missing",
			slackURL:       "https://hooks.slack.com/test",
			discordURL:     "",
			includeDiscord: true,
			expectError:    ErrDiscordWebhookNotSet,
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

			notifiers, err := buildNotifiers(tt.includeDiscord)

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

func TestIncidentCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		service     string
		severity    string
		discord     bool
		slackURL    string
		discordURL  string
		statusCode  int
		expectError bool
		wantErr     error
	}{
		{
			name:        "missing title argument",
			args:        []string{},
			slackURL:    "will-be-set",
			expectError: true,
		},
		{
			name:        "no slack webhook configured",
			args:        []string{"test incident"},
			slackURL:    "",
			discordURL:  "",
			expectError: true,
			wantErr:     ErrSlackWebhookNotSet,
		},
		{
			name:        "successful send to slack only",
			args:        []string{"database is down"},
			service:     "api",
			severity:    "p1",
			slackURL:    "will-be-replaced",
			statusCode:  200,
			expectError: false,
		},
		{
			name:        "successful send to slack and discord",
			args:        []string{"database is down"},
			service:     "api",
			severity:    "p1",
			discord:     true,
			slackURL:    "will-be-replaced",
			discordURL:  "will-be-replaced",
			statusCode:  200,
			expectError: false,
		},
		{
			name:        "discord flag without discord url",
			args:        []string{"database is down"},
			service:     "api",
			severity:    "p1",
			discord:     true,
			slackURL:    "will-be-replaced",
			discordURL:  "",
			statusCode:  200,
			expectError: true,
			wantErr:     ErrDiscordWebhookNotSet,
		},
		{
			name:        "webhook returns error",
			args:        []string{"test"},
			service:     "test",
			severity:    "p1",
			slackURL:    "will-be-replaced",
			statusCode:  500,
			expectError: true,
			wantErr:     ErrAllNotifiersFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incidentService = tt.service
			incidentSeverity = tt.severity
			incidentDiscord = tt.discord
			if incidentSeverity == "" {
				incidentSeverity = "p3"
			}
			if incidentService == "" {
				incidentService = "unknown"
			}
			defer func() {
				incidentService = "unknown"
				incidentSeverity = "p3"
				incidentDiscord = false
			}()

			// Set up a test server if a webhook URL is expected
			if tt.slackURL != "" || tt.discordURL != "" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				}))
				defer server.Close()

				if tt.slackURL != "" {
					t.Setenv("SLACK_WEBHOOK_URL", server.URL)
				} else {
					t.Setenv("SLACK_WEBHOOK_URL", "")
				}
				if tt.discordURL != "" {
					t.Setenv("DISCORD_WEBHOOK_URL", server.URL)
				} else {
					t.Setenv("DISCORD_WEBHOOK_URL", "")
				}
			} else {
				t.Setenv("SLACK_WEBHOOK_URL", "")
				t.Setenv("DISCORD_WEBHOOK_URL", "")
			}

			// cobra.ExactArgs(1) validates before RunE
			err := incidentCmd.Args(incidentCmd, tt.args)
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("unexpected args error: %v", err)
			}

			err = incidentCmd.RunE(incidentCmd, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantErr != nil {
					assert.True(t, errors.Is(err, tt.wantErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
