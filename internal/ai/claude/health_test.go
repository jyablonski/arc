package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func writeClaudeCreds(t *testing.T, home string, expiresAtMs int64, refresh string) {
	t.Helper()
	dir := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	body := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-test","refreshToken":%q,"expiresAt":%d}}`, refresh, expiresAtMs)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(body), 0o600))
}

func healthByCategory(checks []ai.HealthCheck, category string) ai.HealthCheck {
	for _, c := range checks {
		if c.Category == category {
			return c
		}
	}
	return ai.HealthCheck{}
}

func TestProvider_Health_authStatus(t *testing.T) {
	future := time.Now().Add(3 * time.Hour).UnixMilli()
	past := time.Now().Add(-3 * time.Hour).UnixMilli()

	cases := []struct {
		name       string
		writeCreds bool
		expiresAt  int64
		refresh    string
		want       ai.HealthStatus
	}{
		{"valid token", true, future, "rt", ai.HealthOK},
		{"expired but refreshable", true, past, "rt", ai.HealthWarn},
		{"expired without refresh", true, past, "", ai.HealthFail},
		{"missing credentials", false, 0, "", ai.HealthFail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.writeCreds {
				writeClaudeCreds(t, home, tc.expiresAt, tc.refresh)
			}
			auth := healthByCategory((&Provider{HomeDir: home}).Health(context.Background()), "auth")
			require.Equal(t, "claude", auth.Name)
			require.Equal(t, tc.want, auth.Status)
		})
	}
}

func TestProvider_Health_includesToolingCheck(t *testing.T) {
	home := t.TempDir()
	writeClaudeCreds(t, home, time.Now().Add(time.Hour).UnixMilli(), "rt")
	tooling := healthByCategory((&Provider{HomeDir: home}).Health(context.Background()), "tooling")
	require.Equal(t, "claude", tooling.Name)
	require.NotEmpty(t, tooling.Status)
}
