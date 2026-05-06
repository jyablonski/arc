package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
)

const (
	oauthUsageURL   = "https://api.anthropic.com/api/oauth/usage"
	oauthBetaHeader = "oauth-2025-04-20"
	userAgent       = "claude-code/2.0.17"
)

type oauthBucket struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type Provider struct {
	HTTPClient *http.Client
	HomeDir    string
	BaseURL    string
	// OAuthTokenURL overrides POST https://console.anthropic.com/v1/oauth/token (tests).
	OAuthTokenURL string
}

func (p *Provider) Name() string { return "claude" }

func (p *Provider) Usage(ctx context.Context) (ai.UsageReport, error) {
	home := p.HomeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return ai.UsageReport{}, fmt.Errorf("user home: %w", err)
		}
	}
	loaded, err := readOAuthWithMeta(home)
	if err != nil {
		return ai.UsageReport{}, err
	}
	if err := validateOAuthTokenKinds(loaded.AccessToken); err != nil {
		return ai.UsageReport{}, err
	}

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	access := strings.TrimSpace(loaded.AccessToken)
	hasRefresh := strings.TrimSpace(loaded.RefreshToken) != ""

	if exp, ok := parseExpires(loaded.ExpiresAt); ok && time.Now().After(exp) && hasRefresh {
		access, err = p.refreshOAuthCredentials(ctx, home, client)
		if err != nil {
			return ai.UsageReport{}, fmt.Errorf("OAuth token expired and refresh failed: %w", err)
		}
	}

	rep, status, body, err := p.fetchOAuthUsage(ctx, client, access)
	if err != nil {
		return ai.UsageReport{}, err
	}
	if status == http.StatusOK {
		return rep, nil
	}

	if status == http.StatusUnauthorized {
		rel, errR := readOAuthWithMeta(home)
		if errR == nil && strings.TrimSpace(rel.RefreshToken) != "" {
			access, err = p.refreshOAuthCredentials(ctx, home, client)
			if err != nil {
				return ai.UsageReport{}, fmt.Errorf("usage API unauthorized and OAuth refresh failed: %w", err)
			}
			rep, status, body, err = p.fetchOAuthUsage(ctx, client, access)
			if err != nil {
				return ai.UsageReport{}, err
			}
			if status == http.StatusOK {
				return rep, nil
			}
		}
	}

	if status == http.StatusUnauthorized {
		msg := anthropicUserMessage(body)
		if msg != "" {
			return ai.UsageReport{}, fmt.Errorf("usage API 401: %s — subscriber OAuth token not accepted by /api/oauth/usage (often expired); open Claude Code with your Claude subscription so it refreshes ~/.claude/.credentials.json, or confirm auth is OAuth (sk-ant-oat…) not API key-only (sk-ant-api…)", msg)
		}
	}
	url := oauthUsageURL
	if strings.TrimSpace(p.BaseURL) != "" {
		url = strings.TrimSuffix(p.BaseURL, "/") + "/api/oauth/usage"
	}
	return ai.UsageReport{}, fmt.Errorf("usage API %s: %s — body: %s", http.StatusText(status), url, truncate(string(body), 400))
}

func (p *Provider) refreshOAuthCredentials(ctx context.Context, home string, client *http.Client) (string, error) {
	oauthPersistMu.Lock()
	defer oauthPersistMu.Unlock()

	loaded, err := readOAuthWithMeta(home)
	if err != nil {
		return "", err
	}
	rt := strings.TrimSpace(loaded.RefreshToken)
	if rt == "" {
		return "", fmt.Errorf("no OAuth refresh token available")
	}
	if loaded.PersistPath == "" {
		return "", fmt.Errorf("cannot save refreshed OAuth credentials (create ~/.claude/.credentials.json via Claude Code sign-in first)")
	}
	tokenURL := strings.TrimSpace(p.OAuthTokenURL)
	res, err := RefreshOAuthTokens(ctx, RefreshOAuthParams{
		RefreshToken: rt,
		HTTPClient:   client,
		TokenURL:     tokenURL,
	})
	if err != nil {
		return "", err
	}
	if err := mergeRefreshIntoCredentialsFile(loaded.PersistPath, res); err != nil {
		return "", fmt.Errorf("save refreshed OAuth credentials: %w", err)
	}
	fresh, err := readOAuthWithMeta(home)
	if err != nil {
		return "", err
	}
	if err := validateOAuthTokenKinds(fresh.AccessToken); err != nil {
		return "", err
	}
	return strings.TrimSpace(fresh.AccessToken), nil
}

func (p *Provider) fetchOAuthUsage(ctx context.Context, client *http.Client, bearer string) (ai.UsageReport, int, []byte, error) {
	url := oauthUsageURL
	if strings.TrimSpace(p.BaseURL) != "" {
		url = strings.TrimSuffix(p.BaseURL, "/") + "/api/oauth/usage"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ai.UsageReport{}, 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("anthropic-beta", oauthBetaHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return ai.UsageReport{}, 0, nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ai.UsageReport{}, resp.StatusCode, body, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return ai.UsageReport{}, resp.StatusCode, body, fmt.Errorf("decode usage JSON: %w", err)
	}

	var windows []ai.UsageWindow
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val := raw[key]
		if key == "extra_usage" || strings.HasPrefix(key, "_") {
			continue
		}
		var b oauthBucket
		if err := json.Unmarshal(val, &b); err != nil {
			continue
		}
		if b.Utilization == nil && b.ResetsAt == nil {
			continue
		}
		w := ai.UsageWindow{Label: humanizeBucketKey(key)}
		if b.Utilization != nil {
			w.PercentUsed = *b.Utilization
		}
		if b.ResetsAt != nil && *b.ResetsAt != "" {
			if t, err := time.Parse(time.RFC3339, *b.ResetsAt); err == nil {
				w.ResetsAt = &t
			} else {
				w.Detail = "resets_at=" + *b.ResetsAt
			}
		}
		windows = append(windows, w)
	}

	extra := map[string]any{}
	if v, ok := raw["extra_usage"]; ok {
		var eu map[string]any
		if err := json.Unmarshal(v, &eu); err == nil {
			extra["extra_usage"] = eu
		}
	}

	return ai.UsageReport{Windows: windows, Extra: extra}, resp.StatusCode, body, nil
}

func humanizeBucketKey(k string) string {
	switch k {
	case "five_hour":
		return "5 hour"
	case "seven_day":
		return "7 day (all models)"
	case "seven_day_sonnet":
		return "7 day (Sonnet)"
	case "seven_day_opus":
		return "7 day (Opus)"
	case "seven_day_omelette":
		return "7 day (Claude Design)"
	default:
		if strings.HasPrefix(k, "seven_day_") {
			suffix := strings.TrimPrefix(k, "seven_day_")
			if suffix != "" {
				if slug := strings.TrimSpace(strings.ReplaceAll(suffix, "_", " ")); strings.EqualFold(slug, "omelette") {
					return "7 day (Claude Design)"
				}
				w := strings.ReplaceAll(suffix, "_", " ")
				return "7 day (" + sentenceCaseWords(w) + ")"
			}
		}
		return strings.ReplaceAll(k, "_", " ")
	}
}

func sentenceCaseWords(s string) string {
	fields := strings.Fields(strings.ToLower(s))
	for i, f := range fields {
		if f == "" {
			continue
		}
		fields[i] = strings.ToUpper(f[:1]) + f[1:]
	}
	return strings.Join(fields, " ")
}

func anthropicUserMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Error.Message)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
