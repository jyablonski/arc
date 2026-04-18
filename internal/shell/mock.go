package shell

type MockRunner struct {
	RunFunc            func(name string, args ...string) (string, error)
	RunInteractiveFunc func(name string, args ...string) error
	CommandExistsFunc  func(name string) bool
}

var mockRunner *MockRunner

func SetMockRunner(m *MockRunner) {
	mockRunner = m
}

func ClearMockRunner() {
	mockRunner = nil
}
