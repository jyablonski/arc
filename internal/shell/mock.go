package shell

// MockRunner allows mocking shell.Run in tests
type MockRunner struct {
	RunFunc func(name string, args ...string) (string, error)
}

var mockRunner *MockRunner

// SetMockRunner sets a mock runner for testing
// This should only be used in test files
func SetMockRunner(m *MockRunner) {
	mockRunner = m
}

// ClearMockRunner clears the mock runner
func ClearMockRunner() {
	mockRunner = nil
}

// getMockRunner returns the current mock runner (for testing)
func getMockRunner() *MockRunner {
	return mockRunner
}
