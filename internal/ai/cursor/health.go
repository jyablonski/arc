package cursor

import (
	"context"
	"os"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

const cursorAuthHint = "sign in to Cursor; the session token lives in its state.vscdb (cursorAuth/accessToken)"

// Health reports Cursor's offline auth state. Cursor is a GUI app, so there is
// no CLI-on-PATH check; presence of a readable session token is the signal.
func (p *Provider) Health(ctx context.Context) []ai.HealthCheck {
	_ = ctx
	return []ai.HealthCheck{p.authCheck()}
}

func (p *Provider) authCheck() ai.HealthCheck {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "cursor", Status: ai.HealthFail, Detail: err.Error(), Hint: cursorAuthHint}
	}
	dbPath := StateDBPath(home)
	if _, err := os.Stat(dbPath); err != nil {
		return ai.HealthCheck{Category: "auth", Name: "cursor", Status: ai.HealthFail, Detail: "Cursor state DB not found (is Cursor installed?)", Hint: cursorAuthHint}
	}
	tokenRaw, err := ReadAccessTokenFromDB(dbPath)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "cursor", Status: ai.HealthFail, Detail: err.Error(), Hint: cursorAuthHint}
	}
	_, jwt, err := splitSessionToken(tokenRaw)
	if err != nil || strings.TrimSpace(jwt) == "" {
		return ai.HealthCheck{Category: "auth", Name: "cursor", Status: ai.HealthWarn, Detail: "session token present but could not be parsed", Hint: cursorAuthHint}
	}
	exp, ok := ai.JWTExpiry(jwt)
	return ai.TokenExpiryCheck("cursor", exp, ok, false, "state.vscdb", cursorAuthHint)
}
