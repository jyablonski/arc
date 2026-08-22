# Incident alerts

`arc incident` sends a structured incident alert to Slack and, optionally, Discord. It uses incoming webhooks and does not store the webhook URLs.

## Configure webhooks

Set the Slack webhook in your shell before running the command:

```bash
export SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...'
```

Slack is required for every alert. To send the same alert to Discord, also set:

```bash
export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/...'
```

Keep these values in your shell's secret manager or environment setup. Do not commit them to a repository or put them in a script that is shared with others.

## Send an alert

```bash
arc incident "Database outage"
arc incident "Checkout latency" --service payments --severity p2
arc incident "API unavailable" --service api --severity p1 --discord
```

The title is required. `--service` defaults to `unknown`, and `--severity` defaults to `p3`. Valid severities are `p1`, `p2`, and `p3`.

| Flag | Default | Purpose |
| --- | --- | --- |
| `--service` | `unknown` | Identify the affected service. |
| `--severity` | `p3` | Set the alert severity. |
| `--discord` | `false` | Send to Discord as well as Slack. |

Slack messages use a structured block with the title, service, and severity. Discord messages use an embed with the same fields. The command returns an error when every selected destination fails; if one destination succeeds, failures for the others are reported as warnings.
