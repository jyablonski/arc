package claude

import (
	"context"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

const claudeAuthHint = "sign in with Claude Code (Pro/Max); token lives in ~/.claude/.credentials.json or the macOS Keychain"

// Health reports Claude's offline auth state and whether the CLI is installed.
func (p *Provider) Health(ctx context.Context) []ai.HealthCheck {
	_ = ctx
	return []ai.HealthCheck{
		p.authCheck(),
		ai.ToolCheck("claude", "Claude Code CLI"),
	}
}

func (p *Provider) authCheck() ai.HealthCheck {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "claude", Status: ai.HealthFail, Detail: err.Error(), Hint: claudeAuthHint}
	}
	loaded, err := readOAuthWithMeta(home)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "claude", Status: ai.HealthFail, Detail: err.Error(), Hint: claudeAuthHint}
	}
	exp, ok := parseExpires(loaded.ExpiresAt)
	hasRefresh := strings.TrimSpace(loaded.RefreshToken) != ""
	return ai.TokenExpiryCheck("claude", exp, ok, hasRefresh, shortenHome(loaded.CredsPath, home), claudeAuthHint)
}

// shortenHome collapses a leading home-directory prefix to "~" so the source
// label stays compact (a Keychain label has no such prefix and is untouched).
func shortenHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
