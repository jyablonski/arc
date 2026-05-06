package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	oauthTokenURLDefault  = "https://console.anthropic.com/v1/oauth/token"
	oauthClientIDDefault  = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	jsonContentTypeOAuth  = "application/json"
	userAgentOAuthRefresh = "arc-claude-oauth-refresh"
)

// RefreshOAuthParams holds Anthropic OAuth refresh-token exchange parameters.
type RefreshOAuthParams struct {
	RefreshToken string
	ClientID     string // empty uses default client ID
	TokenURL     string // empty uses default token endpoint

	HTTPClient *http.Client
	// Now sets the clock for token expiry; nil uses time.Now.
	Now func() time.Time
}

// RefreshOAuthResult is written into claudeAiOauth in ~/.claude/.credentials.json.
type RefreshOAuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type refreshTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshOAuthTokens exchanges a refresh token for new OAuth tokens.
func RefreshOAuthTokens(ctx context.Context, p RefreshOAuthParams) (RefreshOAuthResult, error) {
	var zero RefreshOAuthResult
	rt := strings.TrimSpace(p.RefreshToken)
	if rt == "" {
		return zero, fmt.Errorf("refresh token empty")
	}
	url := strings.TrimSpace(p.TokenURL)
	if url == "" {
		url = oauthTokenURLDefault
	}
	clientID := strings.TrimSpace(p.ClientID)
	if clientID == "" {
		clientID = oauthClientIDDefault
	}
	nowFn := p.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	payload := refreshTokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: rt,
		ClientID:     clientID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("encode refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", jsonContentTypeOAuth)
	req.Header.Set("Accept", jsonContentTypeOAuth)
	req.Header.Set("User-Agent", userAgentOAuthRefresh)

	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return zero, fmt.Errorf("oauth token %s (%s): %s", resp.Status, url, truncateOAuthErr(string(respBody), 480))
	}

	var decoded refreshTokenResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return zero, fmt.Errorf("decode oauth token response: %w", err)
	}
	at := strings.TrimSpace(decoded.AccessToken)
	if at == "" {
		return zero, fmt.Errorf("oauth token response missing access_token")
	}
	outRT := strings.TrimSpace(decoded.RefreshToken)
	if outRT == "" {
		outRT = rt
	}
	sec := decoded.ExpiresIn
	if sec <= 0 {
		sec = 3600
	}
	exp := nowFn().Add(time.Duration(sec) * time.Second)
	return RefreshOAuthResult{
		AccessToken:  at,
		RefreshToken: outRT,
		ExpiresAt:    exp,
	}, nil
}

func truncateOAuthErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
