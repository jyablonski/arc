package shell

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetMockRunner(t *testing.T) {
	// Test that SetMockRunner works
	mock := &MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			return "mocked", nil
		},
	}

	SetMockRunner(mock)
	defer ClearMockRunner()

	// Verify mock is set by calling Run with mock
	result, err := Run("test", "arg")
	require.NoError(t, err)
	assert.Equal(t, "mocked", result)
}

func TestClearMockRunner(t *testing.T) {
	// Set a mock
	mock := &MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			return "mocked", nil
		},
	}
	SetMockRunner(mock)

	// Clear it
	ClearMockRunner()

	// Verify mock is cleared
	assert.Nil(t, mockRunner, "ClearMockRunner() should clear mockRunner")
}

func TestMockRunnerIntegration(t *testing.T) {
	// Test that mock runner integrates properly with Run
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
