package arcerrs

import "errors"

var (
	ErrSlackWebhookNotSet = errors.New("SLACK_WEBHOOK_URL is not set")

	ErrDiscordWebhookNotSet = errors.New("DISCORD_WEBHOOK_URL is not set")

	ErrAllNotifiersFailed = errors.New("failed to send incident to all configured destinations")

	ErrValidationFailed = errors.New("validation failed: missing required tools")

	ErrNotGitRepo = errors.New("not in a git repository")

	ErrEmptyProviderFilter = errors.New("--provider: empty after parsing")

	ErrHealthCheckFailed = errors.New("one or more AI health checks failed")

	ErrAUROnlyLinuxOnly = errors.New("--aur-only is only supported on Linux")

	ErrNoAURLinuxOnly = errors.New("--no-aur is only supported on Linux")

	ErrAURLinuxOnly = errors.New("--aur is only supported on Linux")

	ErrCaskDarwinOnly = errors.New("--cask is only supported on macOS")

	ErrFormulaDarwinOnly = errors.New("--formula is only supported on macOS")

	ErrInstallJSONUnsupported = errors.New("--json is not supported for interactive package installation")

	ErrAssumeYesLinuxOnly = errors.New("--yes is only supported for system updates on Linux")

	ErrUpdateLogLinuxOnly = errors.New("--log is only supported for system updates on Linux")

	ErrAURDiffLinuxOnly = errors.New("--diff is only supported for system updates on Linux")
)
