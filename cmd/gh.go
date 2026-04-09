package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

const (
	ghRepo         = "jyablonski/nba_elt_dashboard"
	ghWorkflowFile = "vm_cron_restart.yml"
	ghRef          = "master"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "GitHub workflow management",
	Long:  `Manage GitHub workflows. Use subcommands to perform specific actions.`,
}

var ghRestartDashboardCmd = &cobra.Command{
	Use:   "restart-dashboard",
	Short: "Restart the dashboard GitHub workflow",
	Long:  `Trigger the vm_cron_restart.yml workflow in the nba_elt_dashboard repository and wait for completion.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureCommandEnabled(cmd); err != nil {
			return err
		}

		if !shell.CommandExists("gh") {
			return shell.NewErrToolNotAvailable("gh")
		}

		output.Info("Triggering GitHub workflow...")
		if _, err := shell.Run("gh", "workflow", "run", ghWorkflowFile, "--repo", ghRepo, "--ref", ghRef); err != nil {
			return fmt.Errorf("failed to trigger workflow: %w", err)
		}

		output.Success("Workflow triggered successfully")

		// Wait a moment for the workflow run to be created
		output.Info("Waiting for workflow run to start...")
		time.Sleep(2 * time.Second)

		// Get the most recent workflow run ID
		runID, err := getLatestWorkflowRunID()
		if err != nil {
			return fmt.Errorf("failed to get workflow run ID: %w", err)
		}

		output.Info(fmt.Sprintf("Monitoring workflow run %s...", runID))

		// Poll for workflow completion
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
	},
}

// getLatestWorkflowRunID fetches the most recent workflow run ID for the workflow
func getLatestWorkflowRunID() (string, error) {
	result, err := shell.Run("gh", "api",
		fmt.Sprintf("repos/%s/actions/workflows/%s/runs", ghRepo, ghWorkflowFile),
		"--jq", ".workflow_runs[0].id")
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", ErrNoWorkflowRuns
	}
	return result, nil
}

// waitForWorkflowCompletion polls the workflow run until it completes
func waitForWorkflowCompletion(runID string) (status, conclusion string, err error) {
	const (
		pollInterval = 10 * time.Second
		maxWait      = 2 * time.Minute
	)

	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		result, err := shell.Run("gh", "api",
			fmt.Sprintf("repos/%s/actions/runs/%s", ghRepo, runID),
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

func init() {
	rootCmd.AddCommand(ghCmd)
	ghCmd.AddCommand(ghRestartDashboardCmd)
	configureAdminCommands()
}
