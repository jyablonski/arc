package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCmd(t *testing.T) {
	tests := []struct {
		name           string
		availableTools map[string]bool
		expectError    bool
	}{
		{
			name: "all required tools available",
			availableTools: map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "uv": true,
			},
			expectError: false,
		},
		{
			name: "missing required tool",
			availableTools: map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "uv": false,
			},
			expectError: true,
		},
		{
			name:           "all required tools missing",
			availableTools: map[string]bool{},
			expectError:    true,
		},
		{
			name: "required present optional missing",
			availableTools: map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "uv": true,
				// optional tools missing - should still pass
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				CommandExistsFunc: func(name string) bool {
					return tt.availableTools[name]
				},
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			err := validateCmd.RunE(validateCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "missing required tools")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateToolStatusCategories(t *testing.T) {
	// Verify the command correctly categorizes required vs optional tools
	mock := &shell.MockRunner{
		CommandExistsFunc: func(name string) bool {
			// Only required tools are available
			required := map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "uv": true,
			}
			return required[name]
		},
	}
	shell.SetMockRunner(mock)
	defer shell.ClearMockRunner()

	// Should succeed even though optional tools (docker, yay, aws, etc.) are missing
	err := validateCmd.RunE(validateCmd, []string{})
	require.NoError(t, err)
}
