package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyablonski/arc/internal/ai"
)

const codexAuthHint = "run 'codex login'; auth is stored in ~/.codex/auth.json"

type codexAuthFile struct {
	AuthMode     string `json:"auth_mode"`
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
	Tokens       *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"tokens"`
}

// Health reports Codex's offline auth state and whether the CLI is installed.
func (p *Provider) Health(ctx context.Context) []ai.HealthCheck {
	_ = ctx
	bin := p.CodexBinary
	if strings.TrimSpace(bin) == "" {
		bin = "codex"
	}
	return []ai.HealthCheck{
		p.authCheck(),
		ai.ToolCheck(bin, "Codex CLI"),
	}
}

func (p *Provider) authCheck() ai.HealthCheck {
	home, err := ai.ResolveHomeDir(p.HomeDir)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: err.Error(), Hint: codexAuthHint}
	}
	path := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: fmt.Sprintf("no auth at %s", path), Hint: codexAuthHint}
	}
	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: fmt.Sprintf("cannot parse %s: %v", path, err), Hint: codexAuthHint}
	}

	if strings.EqualFold(auth.AuthMode, "apikey") || (auth.Tokens == nil && auth.OpenAIAPIKey != "") {
		if strings.TrimSpace(auth.OpenAIAPIKey) == "" {
			return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: "api-key auth selected but OPENAI_API_KEY is empty", Hint: codexAuthHint}
		}
		return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthOK, Detail: "authenticated with API key"}
	}

	if auth.Tokens == nil || strings.TrimSpace(auth.Tokens.AccessToken) == "" {
		return ai.HealthCheck{Category: "auth", Name: "codex", Status: ai.HealthFail, Detail: "no access token in auth.json", Hint: codexAuthHint}
	}
	exp, ok := ai.JWTExpiry(auth.Tokens.AccessToken)
	hasRefresh := strings.TrimSpace(auth.Tokens.RefreshToken) != ""
	return ai.TokenExpiryCheck("codex", exp, ok, hasRefresh, "ChatGPT auth", codexAuthHint)
}
