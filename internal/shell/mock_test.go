package shell

import (
	"fmt"
	"testing"
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
	if err != nil {
		t.Fatalf("Run() with mock error = %v", err)
	}
	if result != "mocked" {
		t.Errorf("Run() with mock = %q, want %q", result, "mocked")
	}
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

	// Verify mock is cleared - Run should fail with real command (or succeed if command exists)
	// We can't easily test this without knowing what commands exist, but we can verify
	// that ClearMockRunner doesn't panic
	if mockRunner != nil {
		t.Error("ClearMockRunner() did not clear mockRunner")
	}
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

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != tt.expected {
				t.Errorf("Run() = %q, want %q", result, tt.expected)
			}
		})
	}
}
