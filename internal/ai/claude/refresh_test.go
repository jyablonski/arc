package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshOAuthTokens_happyPath(t *testing.T) {
	fixed := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	var gotBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, jsonContentTypeOAuth, r.Header.Get("Content-Type"))
		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		var decoded refreshTokenRequest
		require.NoError(t, json.Unmarshal(gotBody, &decoded))
		require.Equal(t, "refresh_token", decoded.GrantType)
		require.Equal(t, "rt-test", decoded.RefreshToken)
		require.Equal(t, oauthClientIDDefault, decoded.ClientID)

		w.Header().Set("Content-Type", jsonContentTypeOAuth)
		_, err = io.WriteString(w, `{"access_token":"new-at","expires_in":120,"refresh_token":"rt-next"}`)
		require.NoError(t, err)
	}))
	defer ts.Close()

	got, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{
		RefreshToken: "rt-test",
		HTTPClient:   ts.Client(),
		TokenURL:     ts.URL,
		ClientID:     oauthClientIDDefault,
		Now:          func() time.Time { return fixed },
	})
	require.NoError(t, err)
	require.Equal(t, "new-at", got.AccessToken)
	require.Equal(t, "rt-next", got.RefreshToken)
	require.Equal(t, fixed.Add(120*time.Second), got.ExpiresAt)
}

func TestRefreshOAuthTokens_reusesRefreshWhenOmitted(t *testing.T) {
	fixed := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentTypeOAuth)
		_, _ = io.WriteString(w, `{"access_token":"fresh","expires_in":60}`)
	}))
	defer ts.Close()

	got, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{
		RefreshToken: "same-rt",
		HTTPClient:   ts.Client(),
		TokenURL:     ts.URL,
		ClientID:     oauthClientIDDefault,
		Now:          func() time.Time { return fixed },
	})
	require.NoError(t, err)
	require.Equal(t, "fresh", got.AccessToken)
	require.Equal(t, "same-rt", got.RefreshToken)
	require.Equal(t, fixed.Add(60*time.Second), got.ExpiresAt)
}

func TestRefreshOAuthTokens_defaultExpiresIn_whenZero(t *testing.T) {
	fixed := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentTypeOAuth)
		_, _ = io.WriteString(w, `{"access_token":"a","expires_in":0}`)
	}))
	defer ts.Close()

	got, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{
		RefreshToken: "rt",
		HTTPClient:   ts.Client(),
		TokenURL:     ts.URL,
		Now:          func() time.Time { return fixed },
	})
	require.NoError(t, err)
	require.Equal(t, fixed.Add(time.Hour), got.ExpiresAt)
}

func TestRefreshOAuthTokens_httpErrorIncludesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"nope"}`)
	}))
	defer ts.Close()

	_, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{
		RefreshToken: "bad",
		HTTPClient:   ts.Client(),
		TokenURL:     ts.URL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "invalid_grant")
}

func TestRefreshOAuthTokens_missingAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentTypeOAuth)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer ts.Close()

	_, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{
		RefreshToken: "rt",
		HTTPClient:   ts.Client(),
		TokenURL:     ts.URL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing access_token")
}

func TestRefreshOAuthTokens_emptyRefresh(t *testing.T) {
	_, err := RefreshOAuthTokens(context.Background(), RefreshOAuthParams{RefreshToken: "  "})
	require.ErrorContains(t, err, "refresh token empty")
}
