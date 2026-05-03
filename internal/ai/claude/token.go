package claude

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/shell"
)

type oauthCreds struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   any    `json:"expiresAt"`
}

type credentialsFile struct {
	ClaudeAIOAuth *oauthCreds `json:"claudeAiOauth"`
}

func readAccessToken(homeDir string) (string, error) {
	tok, err := readOAuthWithMeta(homeDir)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

type oauthLoaded struct {
	AccessToken string
	CredsPath   string
}

func readOAuthWithMeta(homeDir string) (*oauthLoaded, error) {
	path := filepath.Join(homeDir, ".claude", ".credentials.json")
	if b, err := os.ReadFile(path); err == nil {
		if _, tok, ok := oauthFromBytes(b); ok {
			if err := validateOAuthTokenKinds(tok.AccessToken); err != nil {
				return nil, err
			}
			if exp, ok := parseExpires(tok.ExpiresAt); ok && time.Now().After(exp) {
				return nil, fmt.Errorf("OAuth access token in %s expired at %s — open Claude Code and sign in again (or wait for refresh) then retry", path, exp.UTC().Format(time.RFC3339))
			}
			return &oauthLoaded{AccessToken: strings.TrimSpace(tok.AccessToken), CredsPath: path}, nil
		}
	}

	if runtime.GOOS == "darwin" {
		out, err := shell.Run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
		if err == nil && out != "" {
			out = strings.TrimSpace(out)
			if _, tok, ok := oauthFromBytes([]byte(out)); ok {
				if err := validateOAuthTokenKinds(tok.AccessToken); err != nil {
					return nil, err
				}
				if exp, ok := parseExpires(tok.ExpiresAt); ok && time.Now().After(exp) {
					return nil, fmt.Errorf("OAuth token in macOS Keychain (Claude Code-credentials) expired at %s — open Claude Code to refresh", exp.UTC().Format(time.RFC3339))
				}
				return &oauthLoaded{AccessToken: strings.TrimSpace(tok.AccessToken), CredsPath: "macOS Keychain (Claude Code-credentials)"}, nil
			}
			if strings.HasPrefix(out, "sk-ant-oat") {
				return &oauthLoaded{AccessToken: out, CredsPath: "macOS Keychain (Claude Code-credentials)"}, nil
			}
			if strings.HasPrefix(out, "eyJ") {
				return &oauthLoaded{AccessToken: out, CredsPath: "macOS Keychain (Claude Code-credentials)"}, nil
			}
		}
	}

	return nil, fmt.Errorf("no Claude OAuth token in %s or macOS Keychain (Claude Code-credentials); run Claude Code login", path)
}

func oauthFromBytes(b []byte) (*credentialsFile, *oauthCreds, bool) {
	var cf credentialsFile
	if err := json.Unmarshal(b, &cf); err == nil && cf.ClaudeAIOAuth != nil {
		t := strings.TrimSpace(cf.ClaudeAIOAuth.AccessToken)
		if t != "" {
			return &cf, cf.ClaudeAIOAuth, true
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, nil, false
	}
	v, ok := m["claudeAiOauth"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	s, _ := v["accessToken"].(string)
	if strings.TrimSpace(s) == "" {
		return nil, nil, false
	}
	tok := &oauthCreds{AccessToken: s, ExpiresAt: v["expiresAt"]}
	return nil, tok, true
}

func validateOAuthTokenKinds(access string) error {
	access = strings.TrimSpace(access)
	if strings.HasPrefix(access, "sk-ant-api") {
		return fmt.Errorf("~/.claude/.credentials.json has an API key (sk-ant-api…); the usage endpoint needs a Claude Code OAuth access token (sk-ant-oat…) — use Claude Code with your Pro/Max subscription login, not API-key-only auth")
	}
	return nil
}

func parseExpires(v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || x <= 0 {
			return time.Time{}, false
		}
		if x > 1e12 {
			return time.UnixMilli(int64(x)), true
		}
		return time.Unix(int64(x), 0), true
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return time.Time{}, false
		}
		return parseExpires(f)
	case string:
		if strings.TrimSpace(x) == "" {
			return time.Time{}, false
		}
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
