package cmd

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/extracmd"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

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
			oldValue, hadValue := os.LookupEnv(extracmd.EnvVar)
			defer func() {
				if hadValue {
					_ = os.Setenv(extracmd.EnvVar, oldValue)
				} else {
					_ = os.Unsetenv(extracmd.EnvVar)
				}
				extracmd.ApplyVisibility()
			}()

			if tt.enabled {
				_ = os.Setenv(extracmd.EnvVar, "1")
			} else {
				_ = os.Unsetenv(extracmd.EnvVar)
			}
			extracmd.ApplyVisibility()

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
