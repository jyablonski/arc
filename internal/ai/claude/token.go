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
)

// goos is the OS check used to gate the macOS Keychain fallback. It mirrors
// runtime.GOOS in production but is a package var so tests can exercise the
// darwin branch on any host.
var goos = runtime.GOOS

type oauthCreds struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    any    `json:"expiresAt"`
}

type credentialsFile struct {
	ClaudeAIOAuth *oauthCreds `json:"claudeAiOauth"`
}

type oauthLoaded struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    any
	CredsPath    string // label for errors (file path or Keychain)
	PersistPath  string // writable ~/.claude/.credentials.json when refresh may update disk
}

func readOAuthWithMeta(homeDir string) (*oauthLoaded, error) {
	path := filepath.Join(homeDir, ".claude", ".credentials.json")
	if b, err := os.ReadFile(path); err == nil {
		if _, tok, ok := oauthFromBytes(b); ok {
			return loadOAuthToken(tok, path, path, func(exp time.Time) error {
				return fmt.Errorf("OAuth access token in %s expired at %s — open Claude Code and sign in again (or wait for refresh) then retry", path, exp.UTC().Format(time.RFC3339))
			})
		}
	}

	const keychainLabel = "macOS Keychain (Claude Code-credentials)"
	if goos == "darwin" {
		out, err := run.Run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
		if err == nil && out != "" {
			out = strings.TrimSpace(out)
			if _, tok, ok := oauthFromBytes([]byte(out)); ok {
				// Only persist a refreshed token back to disk when the Keychain
				// payload carried a refresh token to begin with.
				persistPath := ""
				if strings.TrimSpace(tok.RefreshToken) != "" {
					persistPath = path
				}
				return loadOAuthToken(tok, keychainLabel, persistPath, func(exp time.Time) error {
					return fmt.Errorf("OAuth token in %s expired at %s — open Claude Code to refresh", keychainLabel, exp.UTC().Format(time.RFC3339))
				})
			}
			if strings.HasPrefix(out, "sk-ant-oat") || strings.HasPrefix(out, "eyJ") {
				return &oauthLoaded{AccessToken: out, CredsPath: keychainLabel}, nil
			}
		}
	}

	return nil, fmt.Errorf("no Claude OAuth token in %s or %s; run Claude Code login", path, keychainLabel)
}

// loadOAuthToken validates a parsed OAuth token, rejects one that has expired
// with no refresh token to recover it, and otherwise builds the loaded
// credentials. label names the source (file path or Keychain) for errors and
// CredsPath; persistPath is where a refreshed token may be written back (empty
// when there's nowhere safe to persist); expiredErr builds the source-specific
// "token expired" error.
func loadOAuthToken(tok *oauthCreds, label, persistPath string, expiredErr func(exp time.Time) error) (*oauthLoaded, error) {
	if err := validateOAuthTokenKinds(tok.AccessToken); err != nil {
		return nil, err
	}
	if exp, ok := parseExpires(tok.ExpiresAt); ok && time.Now().After(exp) && strings.TrimSpace(tok.RefreshToken) == "" {
		return nil, expiredErr(exp)
	}
	return &oauthLoaded{
		AccessToken:  strings.TrimSpace(tok.AccessToken),
		RefreshToken: strings.TrimSpace(tok.RefreshToken),
		ExpiresAt:    tok.ExpiresAt,
		CredsPath:    label,
		PersistPath:  persistPath,
	}, nil
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
	rt, _ := v["refreshToken"].(string)
	tok := &oauthCreds{AccessToken: s, RefreshToken: rt, ExpiresAt: v["expiresAt"]}
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
