package cursor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func clientForCursorAPI(t *testing.T, ts *httptest.Server) *http.Client {
	t.Helper()
	base, err := url.Parse(ts.URL)
	require.NoError(t, err)

	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			req2.URL.Scheme = base.Scheme
			req2.URL.Host = base.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}
}

func TestBuildCookie_prefixedWhenPlainJWT(t *testing.T) {
	c := buildCookie("user_abc", "plain.jwt.sig")
	require.Equal(t, "WorkosCursorSessionToken=user_abc%3A%3Aplain.jwt.sig", c)
}

func TestBuildCookie_encodedDoubleColonPreserved(t *testing.T) {
	c := buildCookie("ignored", "uid%3A%3Atokenpart")
	require.Equal(t, "WorkosCursorSessionToken=uid%3A%3Atokenpart", c)
}

func TestBuildCookie_rawDoubleColon_encoded(t *testing.T) {
	c := buildCookie("ignored", "user_x::jwtpart")
	require.Equal(t, "WorkosCursorSessionToken=user_x%3A%3Ajwtpart", c)
}

func TestTruncate_usageHelper(t *testing.T) {
	require.Equal(t, "hi", truncate("hi", 10))
	long := strings.Repeat("a", 350)
	out := truncate(long, 300)
	require.Len(t, out, 300+len("…"))
	require.True(t, strings.HasSuffix(out, "…"))
}

func TestCursorGET_ok(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasPrefix(r.URL.Path, "/api/usage"))
		require.Contains(t, r.Header.Get("Cookie"), "WorkosCursorSessionToken=")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	b, err := cursorGET(ctx, clientForCursorAPI(t, ts), "/usage?user=user_x", "user_x", "jwt")
	require.NoError(t, err)
	require.Equal(t, []byte(`{}`), b)
}

func TestCursorGET_nonOK_truncatesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(strings.Repeat("z", 400)))
	}))
	defer ts.Close()

	ctx := context.Background()
	_, err := cursorGET(ctx, clientForCursorAPI(t, ts), "/usage?user=u", "u", "jwt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
	require.Contains(t, err.Error(), strings.Repeat("z", 300))
	require.Contains(t, err.Error(), "…")
}
