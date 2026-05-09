package claude

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/oauth_usage.json
var oauthUsageFixture []byte

func TestProvider_Usage_httptest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/oauth/usage", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, oauthBetaHeader, r.Header.Get("anthropic-beta"))
		require.Equal(t, userAgent, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(oauthUsageFixture)
	}))
	defer ts.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, ".claude", ".credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), filemode.Dir))
	require.NoError(t, os.WriteFile(credPath, []byte(`{"claudeAiOauth":{"accessToken":"test-token"}}`), 0o600))

	p := &Provider{
		HomeDir:    dir,
		HTTPClient: ts.Client(),
		BaseURL:    ts.URL,
	}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.Len(t, rep.Windows, 2)
	require.Equal(t, "5 hour", rep.Windows[0].Label)
	require.InDelta(t, 33.0, rep.Windows[0].PercentUsed, 1e-9)
}

func TestProvider_Usage_refreshAfter401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(b), `"grant_type":"refresh_token"`)
		require.Contains(t, string(b), `"refresh_token":"rt1"`)
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, `{"access_token":"after-refresh","expires_in":3600,"refresh_token":"rt2"}`)
		require.NoError(t, err)
	})
	mux.HandleFunc("/api/oauth/usage", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.Contains(auth, "after-refresh") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(oauthUsageFixture)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, ".claude", ".credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), filemode.Dir))
	require.NoError(t, os.WriteFile(credPath, []byte(`{
  "claudeAiOauth": {
    "accessToken": "stale",
    "refreshToken": "rt1",
    "expiresAt": 999999999999999
  }
}`), 0o600))

	p := &Provider{
		HomeDir:       dir,
		HTTPClient:    ts.Client(),
		BaseURL:       ts.URL,
		OAuthTokenURL: ts.URL + "/v1/oauth/token",
	}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.Len(t, rep.Windows, 2)

	b, err := os.ReadFile(credPath)
	require.NoError(t, err)
	require.Contains(t, string(b), "after-refresh")
	require.Contains(t, string(b), "rt2")
}

func TestProvider_Usage_401_noRefresh_includesAnthropicMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/oauth/usage", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"token revoked"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, ".claude", ".credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), filemode.Dir))
	require.NoError(t, os.WriteFile(credPath, []byte(`{"claudeAiOauth":{"accessToken":"only-access"}}`), 0o600))

	p := &Provider{
		HomeDir:    dir,
		HTTPClient: ts.Client(),
		BaseURL:    ts.URL,
	}
	_, err := p.Usage(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage API 401:")
	require.Contains(t, err.Error(), "token revoked")
	require.Contains(t, err.Error(), "subscriber OAuth token not accepted")
}

func TestProvider_Usage_proactiveRefreshWhenExpired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"after-refresh","expires_in":3600}`)
	})
	mux.HandleFunc("/api/oauth/usage", func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Authorization"), "after-refresh")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(oauthUsageFixture)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	expired := time.Now().Add(-2 * time.Hour).UnixMilli()
	dir := t.TempDir()
	credPath := filepath.Join(dir, ".claude", ".credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), filemode.Dir))
	require.NoError(t, os.WriteFile(credPath, []byte(fmt.Sprintf(`{
  "claudeAiOauth": {
    "accessToken": "old",
    "refreshToken": "rt1",
    "expiresAt": %s
  }
}`, strconv.FormatInt(expired, 10))), 0o600))

	p := &Provider{
		HomeDir:       dir,
		HTTPClient:    ts.Client(),
		BaseURL:       ts.URL,
		OAuthTokenURL: ts.URL + "/v1/oauth/token",
	}
	_, err := p.Usage(context.Background())
	require.NoError(t, err)

	b, err := os.ReadFile(credPath)
	require.NoError(t, err)
	require.Contains(t, string(b), "after-refresh")
}
