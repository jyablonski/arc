package ghworkflow

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestRestartDashboard_ghNotAvailable(t *testing.T) {
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "gh" },
	})
	t.Cleanup(shell.ClearMockRunner)

	err := RestartDashboard()
	var want *shell.ErrToolNotAvailable
	require.ErrorAs(t, err, &want)
	require.Equal(t, "gh", want.Tool)
}

func TestRestartDashboard_workflowTriggerFails(t *testing.T) {
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "gh" },
		RunFunc: func(name string, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "workflow" {
				return "", fmt.Errorf("no permission")
			}
			return "", fmt.Errorf("unexpected: %v", args)
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	err := RestartDashboard()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to trigger workflow")
}

func TestRestartDashboard_success(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps 2s waiting for workflow polling window")
	}
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "gh" },
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "gh", name)
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "workflow run"):
				return "", nil
			case strings.Contains(joined, "workflow_runs"):
				return "999", nil
			case strings.Contains(joined, "/actions/runs/999"):
				return `{"status":"completed","conclusion":"success"}`, nil
			default:
				t.Fatalf("unexpected gh invocation: %v", args)
			}
			return "", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	start := time.Now()
	err := RestartDashboard()
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 2*time.Second)
}

func TestRestartDashboard_workflowFailedConclusion(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps 2s waiting for workflow polling window")
	}
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "gh" },
		RunFunc: func(name string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "workflow run"):
				return "", nil
			case strings.Contains(joined, "workflow_runs"):
				return "111", nil
			case strings.Contains(joined, "/actions/runs/111"):
				return `{"status":"completed","conclusion":"failure"}`, nil
			default:
				t.Fatalf("unexpected gh invocation: %v", args)
			}
			return "", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	err := RestartDashboard()
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow failed with conclusion: failure")
}

func TestRestartDashboard_latestRunIDFailsAfterTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps 2s waiting for workflow polling window")
	}
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "gh" },
		RunFunc: func(name string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "workflow run") {
				return "", nil
			}
			if strings.Contains(joined, "workflow_runs") {
				return "", errors.New("rate limited")
			}
			t.Fatalf("unexpected gh invocation: %v", args)
			return "", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	err := RestartDashboard()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get workflow run ID")
}
