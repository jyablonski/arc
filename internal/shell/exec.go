package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes a command and returns its output
// If a mock runner is set (in tests), it will use the mock instead
func Run(name string, args ...string) (string, error) {
	// Check if we have a mock runner set (for testing)
	if mockRunner != nil && mockRunner.RunFunc != nil {
		return mockRunner.RunFunc(name, args...)
	}

	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunSudo executes a command with sudo and returns its output
func RunSudo(name string, args ...string) (string, error) {
	sudoArgs := append([]string{name}, args...)
	return Run("sudo", sudoArgs...)
}

// RunInteractive executes a command that needs an interactive TTY
func RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CommandExists checks if a command exists in PATH
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
