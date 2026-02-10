package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandExists(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "ls should exist",
			command:  "ls",
			expected: true,
		},
		{
			name:     "nonexistent command should not exist",
			command:  "nonexistent_command_xyz123",
			expected: false,
		},
		{
			name:     "echo should exist",
			command:  "echo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CommandExists(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{
			name:    "echo hello",
			command: "echo",
			args:    []string{"hello"},
			wantErr: false,
		},
		{
			name:    "true command",
			command: "true",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "false command should error",
			command: "false",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "nonexistent command should error",
			command: "nonexistent_command_xyz123",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Run(tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.command == "echo" {
				assert.Contains(t, output, "hello")
			}
		})
	}
}

func TestRunWithOutput(t *testing.T) {
	output, err := Run("echo", "test", "output")
	require.NoError(t, err)
	assert.Equal(t, "test output", output)
}

func TestRunWithWhitespace(t *testing.T) {
	output, err := Run("echo", "  hello  ")
	require.NoError(t, err)
	// TrimSpace removes leading/trailing whitespace from output
	assert.Equal(t, "hello", output)
}
