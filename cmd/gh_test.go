package cmd

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestGetLatestWorkflowRunID(t *testing.T) {
	tests := []struct {
		name        string
		mockRun     func(name string, args ...string) (string, error)
		expectedID  string
		expectError bool
		errContains string
		wantErr     error
	}{
		{
			name: "successful run ID retrieval",
			mockRun: func(name string, args ...string) (string, error) {
				return "12345678", nil
			},
			expectedID:  "12345678",
			expectError: false,
		},
		{
			name: "empty result returns error",
			mockRun: func(name string, args ...string) (string, error) {
				return "", nil
			},
			expectError: true,
			wantErr:     ErrNoWorkflowRuns,
		},
		{
			name: "API call fails",
			mockRun: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("API error")
			},
			expectError: true,
			errContains: "API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: tt.mockRun,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			runID, err := getLatestWorkflowRunID()

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantErr != nil {
					assert.True(t, errors.Is(err, tt.wantErr))
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, runID)
			}
		})
	}
}

func TestWaitForWorkflowCompletion(t *testing.T) {
	tests := []struct {
		name               string
		mockRun            func(name string, args ...string) (string, error)
		expectedStatus     string
		expectedConclusion string
		expectError        bool
		errContains        string
	}{
		{
			name: "workflow completes successfully",
			mockRun: func(name string, args ...string) (string, error) {
				return `{"status": "completed", "conclusion": "success"}`, nil
			},
			expectedStatus:     "completed",
			expectedConclusion: "success",
			expectError:        false,
		},
		{
			name: "workflow completes with failure",
			mockRun: func(name string, args ...string) (string, error) {
				return `{"status": "completed", "conclusion": "failure"}`, nil
			},
			expectedStatus:     "completed",
			expectedConclusion: "failure",
			expectError:        false,
		},
		{
			name: "API call fails",
			mockRun: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("network error")
			},
			expectError: true,
			errContains: "network error",
		},
		{
			name: "invalid JSON response",
			mockRun: func(name string, args ...string) (string, error) {
				return "not json", nil
			},
			expectError: true,
			errContains: "failed to parse workflow run status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: tt.mockRun,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			status, conclusion, err := waitForWorkflowCompletion("12345")

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, status)
				assert.Equal(t, tt.expectedConclusion, conclusion)
			}
		})
	}
}

func TestGhRestartDashboardCmd(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		mockRun     func(name string, args ...string) (string, error)
		mockCmdExst func(name string) bool
		expectError bool
		errContains string
		wantToolErr bool
	}{
		{
			name:        "command not enabled",
			enabled:     false,
			expectError: true,
			errContains: `command "gh restart-dashboard" is not available`,
		},
		{
			name:    "gh not available",
			enabled: true,
			mockCmdExst: func(name string) bool {
				return false
			},
			expectError: true,
			wantToolErr: true,
		},
		{
			name:    "workflow trigger fails",
			enabled: true,
			mockCmdExst: func(name string) bool {
				return name == "gh"
			},
			mockRun: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("trigger failed")
			},
			expectError: true,
			errContains: "failed to trigger workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue, hadValue := os.LookupEnv(extraCommandsEnvVar)
			defer func() {
				if hadValue {
					_ = os.Setenv(extraCommandsEnvVar, oldValue)
				} else {
					_ = os.Unsetenv(extraCommandsEnvVar)
				}
				configureAdminCommands()
			}()

			if tt.enabled {
				_ = os.Setenv(extraCommandsEnvVar, "1")
			} else {
				_ = os.Unsetenv(extraCommandsEnvVar)
			}
			configureAdminCommands()

			mock := &shell.MockRunner{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExst,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := ghRestartDashboardCmd.RunE(ghRestartDashboardCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantToolErr {
					var toolErr *shell.ErrToolNotAvailable
					assert.True(t, errors.As(err, &toolErr))
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
