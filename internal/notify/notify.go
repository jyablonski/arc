package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookError is returned when a webhook endpoint returns a non-success HTTP status.
type WebhookError struct {
	StatusCode int
}

func (e *WebhookError) Error() string {
	return fmt.Sprintf("webhook returned status %d", e.StatusCode)
}

// Incident represents an incident alert to be sent
type Incident struct {
	Title    string
	Service  string
	Severity string
}

// Notifier sends incident alerts to a webhook destination
type Notifier interface {
	Send(incident Incident) error
	Name() string
}

type slackNotifier struct {
	webhookURL string
	client     *http.Client
}

type discordNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlack(webhookURL string) Notifier {
	return &slackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func NewDiscord(webhookURL string) Notifier {
	return &discordNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func severityEmoji(severity string) string {
	switch severity {
	case "p1":
		return "🔴"
	case "p2":
		return "🟠"
	case "p3":
		return "🟡"
	default:
		return "⚪"
	}
}

func severityLabel(severity string) string {
	switch severity {
	case "p1":
		return "P1 - Critical"
	case "p2":
		return "P2 - High"
	case "p3":
		return "P3 - Medium"
	default:
		return severity
	}
}

// Send posts an incident to a Slack incoming webhook
func (s *slackNotifier) Send(incident Incident) error {
	emoji := severityEmoji(incident.Severity)
	label := severityLabel(incident.Severity)

	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("%s Incident: %s", emoji, incident.Title),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("*Service:*\n%s", incident.Service)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Severity:*\n%s", label)},
				},
			},
		},
	}

	return postJSON(s.client, s.webhookURL, payload)
}

func (s *slackNotifier) Name() string {
	return "Slack"
}

// Send posts an incident to a Discord webhook
func (d *discordNotifier) Send(incident Incident) error {
	emoji := severityEmoji(incident.Severity)
	label := severityLabel(incident.Severity)

	// Discord uses embeds for structured messages
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("%s Incident: %s", emoji, incident.Title),
				"color":       discordSeverityColor(incident.Severity),
				"description": fmt.Sprintf("**Service:** %s\n**Severity:** %s", incident.Service, label),
			},
		},
	}

	return postJSON(d.client, d.webhookURL, payload)
}

func (d *discordNotifier) Name() string {
	return "Discord"
}

func discordSeverityColor(severity string) int {
	switch severity {
	case "p1":
		return 0xDC3545 // red
	case "p2":
		return 0xFD7E14 // orange
	case "p3":
		return 0xFFC107 // yellow
	default:
		return 0x6C757D // gray
	}
}

func postJSON(client *http.Client, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Slack returns 200, Discord returns 204
	if resp.StatusCode >= 300 {
		return &WebhookError{StatusCode: resp.StatusCode}
	}

	return nil
}
