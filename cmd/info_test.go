package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestInfoCmd(t *testing.T) {
	tests := []struct {
		name                string
		commandExists       bool
		runInteractiveErr   error
		expectError         bool
		errContains         string
		wantToolUnavailable bool
	}{
		{
			name:                "fastfetch not installed",
			commandExists:       false,
			expectError:         true,
			wantToolUnavailable: true,
		},
		{
			name:          "fastfetch run succeeds",
			commandExists: true,
			expectError:   false,
		},
		{
			name:              "fastfetch run fails",
			commandExists:     true,
			runInteractiveErr: assert.AnError,
			expectError:       true,
			errContains:       assert.AnError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				CommandExistsFunc: func(name string) bool {
					return tt.commandExists
				},
				RunInteractiveFunc: func(name string, args ...string) error {
					assert.Equal(t, "fastfetch", name)
					assert.Empty(t, args)
					return tt.runInteractiveErr
				},
			}
			setRunner(t, mock)

			err := infoCmd.RunE(infoCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.wantToolUnavailable {
					var toolErr *shell.ErrToolNotAvailable
					assert.True(t, errors.As(err, &toolErr))
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}
