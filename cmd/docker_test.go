package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestDockerCmd(t *testing.T) {
	tests := []struct {
		name        string
		mockRun     func(name string, args ...string) (string, error)
		mockCmdExst func(name string) bool
		expectError bool
		wantToolErr bool
	}{
		{
			name: "docker not available",
			mockCmdExst: func(name string) bool {
				return false
			},
			expectError: true,
			wantToolErr: true,
		},
		{
			name: "successful cleanup",
			mockCmdExst: func(name string) bool {
				return name == "docker"
			},
			mockRun: func(name string, args ...string) (string, error) {
				return "", nil
			},
			expectError: false,
		},
		{
			name: "image prune fails but continues",
			mockCmdExst: func(name string) bool {
				return name == "docker"
			},
			mockRun: func(name string, args ...string) (string, error) {
				if len(args) >= 2 && args[0] == "docker" && args[1] == "image" {
					return "", fmt.Errorf("image prune failed")
				}
				return "", nil
			},
			// docker command continues on individual prune failures (uses Warning, not return)
			expectError: false,
		},
		{
			name: "all prune operations fail but command succeeds",
			mockCmdExst: func(name string) bool {
				return name == "docker"
			},
			mockRun: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("prune failed")
			},
			// All operations use Warning on failure, no hard error returned
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc:           tt.mockRun,
				CommandExistsFunc: tt.mockCmdExst,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := dockerCmd.RunE(dockerCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantToolErr {
					var toolErr *shell.ErrToolNotAvailable
					assert.True(t, errors.As(err, &toolErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
