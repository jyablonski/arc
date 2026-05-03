package claude

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
	require.NoError(t, os.MkdirAll(filepath.Dir(credPath), 0o755))
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
