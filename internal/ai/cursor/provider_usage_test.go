package cursor

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func cursorHomeWithAccessToken(t *testing.T, rawToken string) string {
	t.Helper()
	home := t.TempDir()
	dbPath := StateDBPath(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), filemode.Dir))
	dsn := "file:" + filepath.ToSlash(dbPath)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO ItemTable(key,value) VALUES('cursorAuth/accessToken',?)`, rawToken)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return home
}

func httpClientAllRequestsTo(t *testing.T, ts *httptest.Server) *http.Client {
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

func TestProvider_Name(t *testing.T) {
	require.Equal(t, "cursor", (&Provider{}).Name())
}

func TestProvider_Usage_dashboardPath_ok(t *testing.T) {
	home := cursorHomeWithAccessToken(t, "user_u::signed.jwt.token")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "GetCurrentPeriodUsage"):
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"enabled": true,
				"billingCycleStart": "1700000000000",
				"billingCycleEnd": "1735689600000",
				"planUsage": {
					"limit": 10000,
					"totalSpend": 2500,
					"remaining": 7500,
					"totalPercentUsed": 25
				}
			}`))
		case strings.Contains(r.URL.Path, "GetPlanInfo"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"planInfo":{"planName":"Pro"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	p := &Provider{HomeDir: home, HTTPClient: httpClientAllRequestsTo(t, ts)}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, rep.Windows)
	require.Equal(t, "Pro", rep.Extra["plan"])
}

func TestProvider_Usage_restFallback_whenDashboardUnavailable(t *testing.T) {
	home := cursorHomeWithAccessToken(t, "user_u::signed.jwt.token")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "GetCurrentPeriodUsage"),
			strings.Contains(r.URL.Path, "GetPlanInfo"):
			w.WriteHeader(http.StatusBadGateway)
		case strings.HasPrefix(r.URL.Path, "/api/usage"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gpt-4":{"numRequests":3,"maxRequestUsage":100}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	p := &Provider{HomeDir: home, HTTPClient: httpClientAllRequestsTo(t, ts)}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, rep.Windows)
}

func TestProvider_Usage_restFallback_emptyUsage_addsStripeWindow(t *testing.T) {
	home := cursorHomeWithAccessToken(t, "user_u::signed.jwt.token")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "GetCurrentPeriodUsage"):
			w.WriteHeader(http.StatusBadGateway)
		case strings.HasPrefix(r.URL.Path, "/api/usage"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case strings.HasPrefix(r.URL.Path, "/api/auth/stripe"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":"cus_1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	p := &Provider{HomeDir: home, HTTPClient: httpClientAllRequestsTo(t, ts)}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, rep.Windows)
	require.Equal(t, "billing", rep.Windows[len(rep.Windows)-1].Label)
	st, ok := rep.Extra["stripe"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cus_1", st["customer"])
}

func TestProvider_Usage_bothAPIsFail(t *testing.T) {
	home := cursorHomeWithAccessToken(t, "user_u::signed.jwt.token")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetCurrentPeriodUsage") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/usage") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	p := &Provider{HomeDir: home, HTTPClient: httpClientAllRequestsTo(t, ts)}
	_, err := p.Usage(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "cursor api2")
	require.Contains(t, err.Error(), "fallback")
}
