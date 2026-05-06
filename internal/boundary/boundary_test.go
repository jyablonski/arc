package boundary_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
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

func TestDefaultShell_delegatesToShellMock(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		shell.SetMockRunner(&shell.MockRunner{
			RunFunc: func(name string, args ...string) (string, error) {
				require.Equal(t, "echo", name)
				require.Equal(t, []string{"ok"}, args)
				return "hi", nil
			},
		})
		t.Cleanup(shell.ClearMockRunner)
		out, err := boundary.DefaultShell.Run("echo", "ok")
		require.NoError(t, err)
		require.Equal(t, "hi", out)
	})
	t.Run("RunInteractive", func(t *testing.T) {
		shell.SetMockRunner(&shell.MockRunner{
			RunInteractiveFunc: func(name string, args ...string) error {
				require.Equal(t, "vim", name)
				return nil
			},
		})
		t.Cleanup(shell.ClearMockRunner)
		require.NoError(t, boundary.DefaultShell.RunInteractive("vim"))
	})
	t.Run("CommandExists", func(t *testing.T) {
		shell.SetMockRunner(&shell.MockRunner{
			CommandExistsFunc: func(name string) bool {
				return name == "gh"
			},
		})
		t.Cleanup(shell.ClearMockRunner)
		require.True(t, boundary.DefaultShell.CommandExists("gh"))
		require.False(t, boundary.DefaultShell.CommandExists("missing-tool"))
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
