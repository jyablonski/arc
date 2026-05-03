package arcerrs

import "errors"

var (
	ErrSlackWebhookNotSet = errors.New("SLACK_WEBHOOK_URL is not set")

	ErrDiscordWebhookNotSet = errors.New("DISCORD_WEBHOOK_URL is not set")

	ErrAllNotifiersFailed = errors.New("failed to send incident to all configured destinations")

	ErrNoWorkflowRuns = errors.New("no workflow runs found")

	ErrNoUserARN = errors.New("failed to get user ARN")

	ErrValidationFailed = errors.New("validation failed: missing required tools")

	ErrNotGitRepo = errors.New("not in a git repository")
)
