package cmd

import "errors"

var (
	// ErrSlackWebhookNotSet is returned when the SLACK_WEBHOOK_URL env var is missing.
	ErrSlackWebhookNotSet = errors.New("SLACK_WEBHOOK_URL is not set")

	// ErrDiscordWebhookNotSet is returned when the DISCORD_WEBHOOK_URL env var is missing
	// but the --discord flag was provided.
	ErrDiscordWebhookNotSet = errors.New("DISCORD_WEBHOOK_URL is not set")

	// ErrAllNotifiersFailed is returned when an incident could not be delivered
	// to any of the configured webhook destinations.
	ErrAllNotifiersFailed = errors.New("failed to send incident to all configured destinations")

	// ErrNoWorkflowRuns is returned when no GitHub Actions workflow runs are found.
	ErrNoWorkflowRuns = errors.New("no workflow runs found")

	// ErrNoUserARN is returned when the AWS identity response does not contain a valid ARN.
	ErrNoUserARN = errors.New("failed to get user ARN")

	// ErrValidationFailed is returned when one or more required tools are missing
	// during dependency validation.
	ErrValidationFailed = errors.New("validation failed: missing required tools")

	// ErrNotGitRepo is returned when a git command is run outside of a git repository.
	ErrNotGitRepo = errors.New("not in a git repository")
)
