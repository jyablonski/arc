package shell

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetMockRunner(t *testing.T) {
	t.Run("When setting a mock, it intercepts Run calls", func(t *testing.T) {
		mock := &MockRunner{
			RunFunc: func(name string, args ...string) (string, error) {
				return "mocked", nil
			},
		}

		SetMockRunner(mock)
		defer ClearMockRunner()

		result, err := Run("test", "arg")
		require.NoError(t, err)
		assert.Equal(t, "mocked", result)
	})
}

func TestClearMockRunner(t *testing.T) {
	t.Run("When clearing a mock, it resets mockRunner to nil", func(t *testing.T) {
		mock := &MockRunner{
			RunFunc: func(name string, args ...string) (string, error) {
				return "mocked", nil
			},
		}
		SetMockRunner(mock)

		ClearMockRunner()

		assert.Nil(t, mockRunner, "ClearMockRunner() should clear mockRunner")
	})
}

func TestMockRunnerIntegration(t *testing.T) {
	tests := []struct {
		name     string
		mockFunc func(string, ...string) (string, error)
		command  string
		args     []string
		expected string
		wantErr  bool
	}{
		{
			name: "mock returns success",
			mockFunc: func(name string, args ...string) (string, error) {
				return "success", nil
			},
			command:  "test",
			args:     []string{"arg"},
			expected: "success",
			wantErr:  false,
		},
		{
			name: "mock returns error",
			mockFunc: func(name string, args ...string) (string, error) {
				return "", fmt.Errorf("mock error")
			},
			command:  "test",
			args:     []string{"arg"},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRunner{
				RunFunc: tt.mockFunc,
			}
			SetMockRunner(mock)
			defer ClearMockRunner()

			result, err := Run(tt.command, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMockRunInteractive(t *testing.T) {
	t.Run("When called successfully, it captures command and args", func(t *testing.T) {
		var capturedName string
		var capturedArgs []string

		mock := &MockRunner{
			RunInteractiveFunc: func(name string, args ...string) error {
				capturedName = name
				capturedArgs = args
				return nil
			},
		}
		SetMockRunner(mock)
		defer ClearMockRunner()

		err := RunInteractive("sudo", "pacman", "-Syu")
		require.NoError(t, err)
		assert.Equal(t, "sudo", capturedName)
		assert.Equal(t, []string{"pacman", "-Syu"}, capturedArgs)
	})

	t.Run("When mock returns error, it propagates the error", func(t *testing.T) {
		mock := &MockRunner{
			RunInteractiveFunc: func(name string, args ...string) error {
				return fmt.Errorf("interactive command failed")
			},
		}
		SetMockRunner(mock)
		defer ClearMockRunner()

		err := RunInteractive("sudo", "pacman", "-Syu")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interactive command failed")
	})
}

func TestMockCommandExists(t *testing.T) {
	t.Run("When checking known commands, it returns correct results", func(t *testing.T) {
		mock := &MockRunner{
			CommandExistsFunc: func(name string) bool {
				return name == "pacman" || name == "git"
			},
		}
		SetMockRunner(mock)
		defer ClearMockRunner()

		assert.True(t, CommandExists("pacman"))
		assert.True(t, CommandExists("git"))
		assert.False(t, CommandExists("nonexistent"))
	})
}
