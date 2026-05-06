package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMergeRefreshIntoCredentialsFile_preservesSiblingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", ".credentials.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	initial := `{
  "claudeAiOauth": {
    "accessToken": "a",
    "refreshToken": "r",
    "expiresAt": 1,
    "scopes": ["user:inference"],
    "subscriptionType": "pro"
  }
}`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o600))

	res := RefreshOAuthResult{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.UnixMilli(9_999_000),
	}
	require.NoError(t, mergeRefreshIntoCredentialsFile(path, res))

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(b)
	require.Contains(t, s, `"scopes"`)
	require.Contains(t, s, `"subscriptionType": "pro"`)
	require.Contains(t, s, "new-access")
	require.Contains(t, s, "new-refresh")
}
