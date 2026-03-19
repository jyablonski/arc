package shell

// MockRunner allows mocking shell.Run, RunInteractive, and CommandExists in tests
type MockRunner struct {
	RunFunc            func(name string, args ...string) (string, error)
	RunInteractiveFunc func(name string, args ...string) error
	CommandExistsFunc  func(name string) bool
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
