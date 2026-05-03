package ghworkflow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
)

const (
	Repo         = "jyablonski/nba_elt_dashboard"
	WorkflowFile = "vm_cron_restart.yml"
	Ref          = "master"
)

func RestartDashboard() error {
	if !shell.CommandExists("gh") {
		return shell.NewErrToolNotAvailable("gh")
	}

	output.Info("Triggering GitHub workflow...")
	if _, err := shell.Run("gh", "workflow", "run", WorkflowFile, "--repo", Repo, "--ref", Ref); err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}

	output.Success("Workflow triggered successfully")

	output.Info("Waiting for workflow run to start...")
	time.Sleep(2 * time.Second)

	runID, err := latestWorkflowRunID()
	if err != nil {
		return fmt.Errorf("failed to get workflow run ID: %w", err)
	}

	output.Info(fmt.Sprintf("Monitoring workflow run %s...", runID))

	status, conclusion, err := waitForWorkflowCompletion(runID)
	if err != nil {
		return fmt.Errorf("failed to monitor workflow: %w", err)
	}

	if status == "completed" {
		if conclusion == "success" {
			output.Success("Workflow completed successfully")
			return nil
		}
		output.Error(fmt.Sprintf("Workflow completed with conclusion: %s", conclusion))
		return fmt.Errorf("workflow failed with conclusion: %s", conclusion)
	}

	return fmt.Errorf("workflow ended with unexpected status: %s", status)
}

func latestWorkflowRunID() (string, error) {
	result, err := shell.Run("gh", "api",
		fmt.Sprintf("repos/%s/actions/workflows/%s/runs", Repo, WorkflowFile),
		"--jq", ".workflow_runs[0].id")
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", arcerrs.ErrNoWorkflowRuns
	}
	return result, nil
}

func waitForWorkflowCompletion(runID string) (status, conclusion string, err error) {
	const (
		pollInterval = 10 * time.Second
		maxWait      = 2 * time.Minute
	)

	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		result, err := shell.Run("gh", "api",
			fmt.Sprintf("repos/%s/actions/runs/%s", Repo, runID),
			"--jq", "{status: .status, conclusion: .conclusion}")
		if err != nil {
			return "", "", err
		}

		var run struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}
		if err := json.Unmarshal([]byte(result), &run); err != nil {
			return "", "", fmt.Errorf("failed to parse workflow run status: %w", err)
		}

		if run.Status == "completed" {
			return run.Status, run.Conclusion, nil
		}

		output.Info(fmt.Sprintf("Workflow status: %s, waiting...", run.Status))
		time.Sleep(pollInterval)
	}

	return "", "", fmt.Errorf("timeout waiting for workflow to complete (max wait: %v)", maxWait)
}
