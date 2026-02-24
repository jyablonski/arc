package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSudo(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		args       []string
		mockOutput string
		mockError  error
		wantErr    bool
	}{
		{
			name:       "sudo with echo",
			command:    "echo",
			args:       []string{"test"},
			mockOutput: "test",
			mockError:  nil,
			wantErr:    false,
		},
		{
			name:       "sudo with nonexistent command",
			command:    "nonexistent_command_xyz123",
			args:       []string{},
			mockOutput: "",
			mockError:  assert.AnError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRunner{
				RunFunc: func(name string, args ...string) (string, error) {
					// Verify sudo is called with correct arguments
					assert.Equal(t, "sudo", name)
					assert.Equal(t, tt.command, args[0])
					return tt.mockOutput, tt.mockError
				},
			}
			SetMockRunner(mock)
			defer ClearMockRunner()

			output, err := RunSudo(tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.mockOutput, output)
			}
		})
	}

	t.Run("When constructing command, it correctly prepends sudo", func(t *testing.T) {
		var capturedName string
		var capturedArgs []string

		mock := &MockRunner{
			RunFunc: func(name string, args ...string) (string, error) {
				capturedName = name
				capturedArgs = args
				return "mocked", nil
			},
		}
		SetMockRunner(mock)
		defer ClearMockRunner()

		_, err := RunSudo("echo", "hello", "world")
		require.NoError(t, err)

		assert.Equal(t, "sudo", capturedName)
		assert.Equal(t, []string{"echo", "hello", "world"}, capturedArgs)
	})
}
