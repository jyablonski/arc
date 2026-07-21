package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// makeJWT builds an unsigned JWT carrying the given exp for expiry-decode tests.
func makeJWT(exp int64) string {
	payload, _ := json.Marshal(map[string]int64{"exp": exp})
	seg := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	return seg([]byte(`{"alg":"none"}`)) + "." + seg(payload) + "." + seg([]byte("sig"))
}

func TestJWTExpiry(t *testing.T) {
	t.Run("decodes exp claim", func(t *testing.T) {
		exp := time.Now().Add(48 * time.Hour).Unix()
		got, ok := JWTExpiry(makeJWT(exp))
		require.True(t, ok)
		require.Equal(t, exp, got.Unix())
	})
	t.Run("rejects non-jwt", func(t *testing.T) {
		_, ok := JWTExpiry("not-a-jwt")
		require.False(t, ok)
	})
	t.Run("rejects jwt without exp", func(t *testing.T) {
		_, ok := JWTExpiry(makeJWT(0))
		require.False(t, ok)
	})
}

func TestTokenExpiryCheck(t *testing.T) {
	future := time.Now().Add(2 * time.Hour)
	past := time.Now().Add(-2 * time.Hour)

	cases := []struct {
		name       string
		exp        time.Time
		ok         bool
		hasRefresh bool
		want       HealthStatus
	}{
		{"valid token", future, true, false, HealthOK},
		{"expired but refreshable", past, true, true, HealthWarn},
		{"expired without refresh", past, true, false, HealthFail},
		{"unknown expiry", time.Time{}, false, false, HealthOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TokenExpiryCheck("x", tc.exp, tc.ok, tc.hasRefresh, "src", "hint")
			require.Equal(t, tc.want, got.Status)
			require.Equal(t, "auth", got.Category)
		})
	}
}

func TestRunHealthCheckers_filtersAndAggregates(t *testing.T) {
	claude := &HealthCheckerMock{
		NameFunc:   func() string { return "claude" },
		HealthFunc: func(ctx context.Context) []HealthCheck { return []HealthCheck{{Name: "claude", Status: HealthOK}} },
	}
	codex := &HealthCheckerMock{
		NameFunc:   func() string { return "codex" },
		HealthFunc: func(ctx context.Context) []HealthCheck { return []HealthCheck{{Name: "codex", Status: HealthFail}} },
	}

	all := RunHealthCheckers(context.Background(), []HealthChecker{claude, codex}, nil)
	require.Len(t, all, 2)
	require.True(t, HealthReport{Checks: all}.HasFailure())

	only := RunHealthCheckers(context.Background(), []HealthChecker{claude, codex}, []string{"claude"})
	require.Len(t, only, 1)
	require.Equal(t, "claude", only[0].Name)
	require.False(t, HealthReport{Checks: only}.HasFailure())
}

func TestToolCheck_missingBinaryWarns(t *testing.T) {
	c := ToolCheck("definitely-not-a-real-binary-xyz", "Nope CLI")
	require.Equal(t, HealthWarn, c.Status)
	require.Equal(t, "tooling", c.Category)
}
