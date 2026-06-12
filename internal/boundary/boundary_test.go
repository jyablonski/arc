package boundary_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/require"
)

func TestHTTPDoerMock_roundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ts.Close()

	m := &boundary.HTTPDoerMock{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return ts.Client().Do(req)
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	resp, err := m.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	require.Len(t, m.DoCalls(), 1)
	require.Equal(t, http.MethodGet, m.DoCalls()[0].Req.Method)
}

func TestDefaultShell_isShellRunner(t *testing.T) {
	var r = boundary.DefaultShell
	require.NotNil(t, r)
}

// DefaultShell is a thin passthrough to the real OS exec; this exercises it
// against harmless real commands (no global mock involved).
func TestDefaultShell_realExec(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		out, err := boundary.DefaultShell.Run("echo", "ok")
		require.NoError(t, err)
		require.Equal(t, "ok", out)
	})
	t.Run("CommandExists", func(t *testing.T) {
		require.True(t, boundary.DefaultShell.CommandExists("sh"))
		require.False(t, boundary.DefaultShell.CommandExists("definitely-not-a-real-tool-xyz123"))
	})
}

func TestShellRunnerMock_tracksCalls(t *testing.T) {
	m := &boundary.ShellRunnerMock{
		CommandExistsFunc: func(name string) bool { return name == "x" },
		RunFunc: func(name string, args ...string) (string, error) {
			return "out", nil
		},
		RunInteractiveFunc: func(name string, args ...string) error {
			return errors.New("interactive")
		},
	}
	require.True(t, m.CommandExists("x"))
	require.False(t, m.CommandExists("y"))
	out, err := m.Run("a", "b")
	require.NoError(t, err)
	require.Equal(t, "out", out)
	require.Error(t, m.RunInteractive("z"))

	require.Len(t, m.CommandExistsCalls(), 2)
	require.Equal(t, "x", m.CommandExistsCalls()[0].Name)
	require.Len(t, m.RunCalls(), 1)
	require.Equal(t, "a", m.RunCalls()[0].Name)
	require.Equal(t, []string{"b"}, m.RunCalls()[0].Args)
	require.Len(t, m.RunInteractiveCalls(), 1)
	require.Equal(t, "z", m.RunInteractiveCalls()[0].Name)
}
