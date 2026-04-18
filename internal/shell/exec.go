package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Run(name string, args ...string) (string, error) {
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

func RunSudo(name string, args ...string) (string, error) {
	sudoArgs := append([]string{name}, args...)
	return Run("sudo", sudoArgs...)
}

func RunInteractive(name string, args ...string) error {
	if mockRunner != nil && mockRunner.RunInteractiveFunc != nil {
		return mockRunner.RunInteractiveFunc(name, args...)
	}

	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CommandExists(name string) bool {
	if mockRunner != nil && mockRunner.CommandExistsFunc != nil {
		return mockRunner.CommandExistsFunc(name)
	}

	_, err := exec.LookPath(name)
	return err == nil
}
