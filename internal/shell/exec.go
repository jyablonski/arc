package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func Run(name string, args ...string) (string, error) {
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
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunLogged runs an interactive command while preserving its complete output
// in log. When visible is false, both output streams are log-only.
func RunLogged(log io.Writer, visible bool, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = log
	cmd.Stderr = log
	if visible {
		cmd.Stdout = io.MultiWriter(log, os.Stdout)
		cmd.Stderr = io.MultiWriter(log, os.Stderr)
	}
	return cmd.Run()
}

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
