package cmd

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
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
				"gh": true, "fastfetch": true, "uv": true,
			},
			expectError: false,
		},
		{
			name: "missing required tool",
			availableTools: map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "fastfetch": true, "uv": false,
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
				"gh": true, "fastfetch": true, "uv": true,
				// optional tools missing - should still pass
			},
			expectError: false,
		},
		{
			name: "correctly categorizes required vs optional tools",
			availableTools: map[string]bool{
				"pacman": true, "systemctl": true, "lspci": true,
				"dmidecode": true, "lshw": true, "git": true,
				"gh": true, "fastfetch": true, "uv": true,
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
				assert.True(t, errors.Is(err, arcerrs.ErrValidationFailed))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCmd_darwinTools(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	availableTools := map[string]bool{
		"brew": true, "git": true, "gh": true, "fastfetch": true,
		"uv": true, "system_profiler": true, "pmset": true, "sysctl": true,
	}
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool {
			return availableTools[name]
		},
	})
	defer shell.ClearMockRunner()

	assert.NoError(t, validateCmd.RunE(validateCmd, []string{}))
}
