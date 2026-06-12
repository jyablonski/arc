package boundary

import "github.com/jyablonski/arc/internal/shell"

// DefaultShell delegates to package shell (global mock runner still applies in tests).
type shellBridge struct{}

func (shellBridge) Run(name string, args ...string) (string, error) {
	return shell.Run(name, args...)
}

func (shellBridge) RunSudo(name string, args ...string) (string, error) {
	return shell.RunSudo(name, args...)
}

func (shellBridge) RunInteractive(name string, args ...string) error {
	return shell.RunInteractive(name, args...)
}

func (shellBridge) CommandExists(name string) bool {
	return shell.CommandExists(name)
}

// DefaultShell is the production ShellRunner implementation.
var DefaultShell ShellRunner = shellBridge{}

var _ ShellRunner = shellBridge{}
