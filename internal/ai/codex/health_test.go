package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func jwtWithExp(exp int64) string {
	payload, _ := json.Marshal(map[string]int64{"exp": exp})
	seg := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	return seg([]byte(`{"alg":"none"}`)) + "." + seg(payload) + "." + seg([]byte("sig"))
}

func writeCodexAuth(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".codex")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth.json"), []byte(body), filemode.File))
}

func codexAuthStatus(t *testing.T, home string) ai.HealthCheck {
	t.Helper()
	for _, c := range (&Provider{HomeDir: home}).Health(context.Background()) {
		if c.Category == "auth" {
			return c
		}
	}
	t.Fatal("no auth check returned")
	return ai.HealthCheck{}
}

func TestProvider_Health_authStatus(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).Unix()
	past := time.Now().Add(-1 * time.Hour).Unix()
	chatgpt := func(exp int64) string {
		return fmt.Sprintf(`{"auth_mode":"chatgpt","tokens":{"access_token":%q,"refresh_token":"rt"}}`, jwtWithExp(exp))
	}

	cases := []struct {
		name  string
		write bool
		body  string
		want  ai.HealthStatus
	}{
		{"chatgpt valid token", true, chatgpt(future), ai.HealthOK},
		{"chatgpt expired but refreshable", true, chatgpt(past), ai.HealthWarn},
		{"api key auth", true, `{"auth_mode":"apikey","OPENAI_API_KEY":"sk-test"}`, ai.HealthOK},
		{"api key auth without key", true, `{"auth_mode":"apikey","OPENAI_API_KEY":""}`, ai.HealthFail},
		{"missing auth file", false, "", ai.HealthFail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.write {
				writeCodexAuth(t, home, tc.body)
			}
			auth := codexAuthStatus(t, home)
			require.Equal(t, "codex", auth.Name)
			require.Equal(t, tc.want, auth.Status)
		})
	}
}
