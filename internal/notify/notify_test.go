package notify

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackSend(t *testing.T) {
	tests := []struct {
		name        string
		incident    Incident
		statusCode  int
		expectError bool
		checkBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "p1 incident",
			incident: Incident{
				Title:    "database is down",
				Service:  "api",
				Severity: "p1",
			},
			statusCode:  200,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				blocks := body["blocks"].([]interface{})
				require.Len(t, blocks, 2)

				header := blocks[0].(map[string]interface{})
				assert.Equal(t, "header", header["type"])
				text := header["text"].(map[string]interface{})
				assert.Contains(t, text["text"], "database is down")
				assert.Contains(t, text["text"], "🔴")

				section := blocks[1].(map[string]interface{})
				fields := section["fields"].([]interface{})
				require.Len(t, fields, 2)
				serviceField := fields[0].(map[string]interface{})
				assert.Contains(t, serviceField["text"], "api")
				severityField := fields[1].(map[string]interface{})
				assert.Contains(t, severityField["text"], "P1 - Critical")
			},
		},
		{
			name: "p2 incident",
			incident: Incident{
				Title:    "high latency",
				Service:  "payments",
				Severity: "p2",
			},
			statusCode:  200,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				blocks := body["blocks"].([]interface{})
				header := blocks[0].(map[string]interface{})
				text := header["text"].(map[string]interface{})
				assert.Contains(t, text["text"], "🟠")

				section := blocks[1].(map[string]interface{})
				fields := section["fields"].([]interface{})
				severityField := fields[1].(map[string]interface{})
				assert.Contains(t, severityField["text"], "P2 - High")
			},
		},
		{
			name: "p3 incident",
			incident: Incident{
				Title:    "cert expiring",
				Service:  "infra",
				Severity: "p3",
			},
			statusCode:  200,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				blocks := body["blocks"].([]interface{})
				header := blocks[0].(map[string]interface{})
				text := header["text"].(map[string]interface{})
				assert.Contains(t, text["text"], "🟡")
			},
		},
		{
			name: "webhook returns error status",
			incident: Incident{
				Title:    "test",
				Service:  "test",
				Severity: "p1",
			},
			statusCode:  403,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, http.MethodPost, r.Method)

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &receivedBody))

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			notifier := NewSlack(server.URL)
			assert.Equal(t, "Slack", notifier.Name())

			err := notifier.Send(tt.incident)

			if tt.expectError {
				assert.Error(t, err)
				var webhookErr *WebhookError
				assert.True(t, errors.As(err, &webhookErr))
				assert.Equal(t, tt.statusCode, webhookErr.StatusCode)
			} else {
				assert.NoError(t, err)
				if tt.checkBody != nil {
					tt.checkBody(t, receivedBody)
				}
			}
		})
	}

	t.Run("When server is unreachable, it returns an error", func(t *testing.T) {
		slack := NewSlack("http://localhost:1")
		err := slack.Send(Incident{Title: "test", Service: "test", Severity: "p1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send request")
	})
}

func TestDiscordSend(t *testing.T) {
	tests := []struct {
		name        string
		incident    Incident
		statusCode  int
		expectError bool
		checkBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "p1 incident",
			incident: Incident{
				Title:    "database is down",
				Service:  "api",
				Severity: "p1",
			},
			statusCode:  204,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				embeds := body["embeds"].([]interface{})
				require.Len(t, embeds, 1)

				embed := embeds[0].(map[string]interface{})
				assert.Contains(t, embed["title"], "database is down")
				assert.Contains(t, embed["title"], "🔴")
				assert.Contains(t, embed["description"], "api")
				assert.Contains(t, embed["description"], "P1 - Critical")
				// 0xDC3545 = 14431557
				assert.Equal(t, float64(0xDC3545), embed["color"])
			},
		},
		{
			name: "p2 incident with orange color",
			incident: Incident{
				Title:    "slow queries",
				Service:  "db",
				Severity: "p2",
			},
			statusCode:  204,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				embeds := body["embeds"].([]interface{})
				embed := embeds[0].(map[string]interface{})
				assert.Equal(t, float64(0xFD7E14), embed["color"])
				assert.Contains(t, embed["title"], "🟠")
			},
		},
		{
			name: "unknown severity uses gray",
			incident: Incident{
				Title:    "something happened",
				Service:  "misc",
				Severity: "info",
			},
			statusCode:  204,
			expectError: false,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				embeds := body["embeds"].([]interface{})
				embed := embeds[0].(map[string]interface{})
				assert.Equal(t, float64(0x6C757D), embed["color"])
				assert.Contains(t, embed["title"], "⚪")
			},
		},
		{
			name: "webhook returns error",
			incident: Incident{
				Title:    "test",
				Service:  "test",
				Severity: "p1",
			},
			statusCode:  500,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &receivedBody))

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			notifier := NewDiscord(server.URL)
			assert.Equal(t, "Discord", notifier.Name())

			err := notifier.Send(tt.incident)

			if tt.expectError {
				assert.Error(t, err)
				var webhookErr *WebhookError
				assert.True(t, errors.As(err, &webhookErr))
				assert.Equal(t, tt.statusCode, webhookErr.StatusCode)
			} else {
				assert.NoError(t, err)
				if tt.checkBody != nil {
					tt.checkBody(t, receivedBody)
				}
			}
		})
	}

	t.Run("When server is unreachable, it returns an error", func(t *testing.T) {
		discord := NewDiscord("http://localhost:1")
		err := discord.Send(Incident{Title: "test", Service: "test", Severity: "p1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send request")
	})
}

func TestSeverityEmoji(t *testing.T) {
	t.Run("When severity is p1, it returns red circle", func(t *testing.T) {
		assert.Equal(t, "🔴", severityEmoji("p1"))
	})

	t.Run("When severity is p2, it returns orange circle", func(t *testing.T) {
		assert.Equal(t, "🟠", severityEmoji("p2"))
	})

	t.Run("When severity is p3, it returns yellow circle", func(t *testing.T) {
		assert.Equal(t, "🟡", severityEmoji("p3"))
	})

	t.Run("When severity is unknown, it returns white circle", func(t *testing.T) {
		assert.Equal(t, "⚪", severityEmoji("unknown"))
	})

	t.Run("When severity is empty, it returns white circle", func(t *testing.T) {
		assert.Equal(t, "⚪", severityEmoji(""))
	})
}

func TestSeverityLabel(t *testing.T) {
	t.Run("When severity is p1, it returns Critical", func(t *testing.T) {
		assert.Equal(t, "P1 - Critical", severityLabel("p1"))
	})

	t.Run("When severity is p2, it returns High", func(t *testing.T) {
		assert.Equal(t, "P2 - High", severityLabel("p2"))
	})

	t.Run("When severity is p3, it returns Medium", func(t *testing.T) {
		assert.Equal(t, "P3 - Medium", severityLabel("p3"))
	})

	t.Run("When severity is unrecognized, it returns as-is", func(t *testing.T) {
		assert.Equal(t, "info", severityLabel("info"))
	})

	t.Run("When severity is empty, it returns empty", func(t *testing.T) {
		assert.Equal(t, "", severityLabel(""))
	})
}

func TestDiscordSeverityColor(t *testing.T) {
	t.Run("When severity is p1, it returns red", func(t *testing.T) {
		assert.Equal(t, 0xDC3545, discordSeverityColor("p1"))
	})

	t.Run("When severity is p2, it returns orange", func(t *testing.T) {
		assert.Equal(t, 0xFD7E14, discordSeverityColor("p2"))
	})

	t.Run("When severity is p3, it returns yellow", func(t *testing.T) {
		assert.Equal(t, 0xFFC107, discordSeverityColor("p3"))
	})

	t.Run("When severity is unknown, it returns gray", func(t *testing.T) {
		assert.Equal(t, 0x6C757D, discordSeverityColor("unknown"))
	})
}
