package cursor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func clientForDashboardAPI(t *testing.T, ts *httptest.Server) *http.Client {
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

func TestConnectPOST_transportError(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("offline")
		}),
	}
	st, b, err := connectPOST(ctx, client, "tok", "/aiserver.v1.DashboardService/GetCurrentPeriodUsage", []byte("{}"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "POST ")
	require.Contains(t, err.Error(), "offline")
	require.Equal(t, 0, st)
	require.Nil(t, b)
}

func TestFetchDashboardAndPlan_ok(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer jwt-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/aiserver.v1.DashboardService/GetCurrentPeriodUsage":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"enabled":true,"planUsage":{"limit":100}}`))
		case "/aiserver.v1.DashboardService/GetPlanInfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"planInfo":{"planName":"Pro"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	dash, plan, ok, err := fetchDashboardAndPlan(ctx, clientForDashboardAPI(t, ts), "jwt-test")
	require.NoError(t, err)
	require.NotNil(t, dash)
	require.Equal(t, "Pro", plan)
	require.True(t, ok)
}

func TestFetchDashboardAndPlan_firstPOST_non2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		body := strings.Repeat("x", 500)
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, _, err := fetchDashboardAndPlan(ctx, clientForDashboardAPI(t, ts), "tok")
	require.Error(t, err)
	require.Contains(t, err.Error(), "GetCurrentPeriodUsage: HTTP 502")
	require.Contains(t, err.Error(), strings.Repeat("x", 400))
	require.Contains(t, err.Error(), "…")
}

func TestFetchDashboardAndPlan_invalidDashboardJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/aiserver.v1.DashboardService/GetCurrentPeriodUsage", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, _, err := fetchDashboardAndPlan(ctx, clientForDashboardAPI(t, ts), "tok")
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode dashboard usage")
}
