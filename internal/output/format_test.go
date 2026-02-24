package output

import (
	"bytes"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeader(t *testing.T) {
	t.Run("When called, it outputs the header text with separator", func(t *testing.T) {
		color.NoColor = true
		defer func() { color.NoColor = false }()

		stdout, stderr := captureOutput(t, func() {
			Header("Test Header")
		})
		combined := stdout + stderr

		assert.Contains(t, combined, "Test Header")
		assert.Contains(t, stdout, "---")
	})
}

func TestSuccess(t *testing.T) {
	t.Run("When called, it outputs checkmark with message", func(t *testing.T) {
		color.NoColor = true
		defer func() { color.NoColor = false }()

		_, stderr := captureOutput(t, func() {
			Success("Test success")
		})

		assert.Contains(t, stderr, "✓")
		assert.Contains(t, stderr, "Test success")
	})
}

func TestError(t *testing.T) {
	t.Run("When called, it outputs X mark with message", func(t *testing.T) {
		color.NoColor = true
		defer func() { color.NoColor = false }()

		_, stderr := captureOutput(t, func() {
			Error("Test error")
		})

		assert.Contains(t, stderr, "✗")
		assert.Contains(t, stderr, "Test error")
	})
}

func TestInfo(t *testing.T) {
	t.Run("When called, it outputs info marker with message", func(t *testing.T) {
		color.NoColor = true
		defer func() { color.NoColor = false }()

		_, stderr := captureOutput(t, func() {
			Info("Test info")
		})

		assert.Contains(t, stderr, "i ")
		assert.Contains(t, stderr, "Test info")
	})
}

func TestWarning(t *testing.T) {
	t.Run("When called, it outputs warning marker with message", func(t *testing.T) {
		color.NoColor = true
		defer func() { color.NoColor = false }()

		_, stderr := captureOutput(t, func() {
			Warning("Test warning")
		})

		assert.Contains(t, stderr, "⚠")
		assert.Contains(t, stderr, "Test warning")
	})
}

func TestPrint(t *testing.T) {
	t.Run("When called, it outputs text to stdout", func(t *testing.T) {
		stdout, _ := captureOutput(t, func() {
			Print("Test print")
		})

		assert.Contains(t, stdout, "Test print")
	})
}

func TestTable(t *testing.T) {
	t.Run("When called with headers and rows, it outputs formatted table", func(t *testing.T) {
		headers := []string{"Name", "Size"}
		rows := [][]string{
			{"package1", "100 MiB"},
			{"package2", "200 MiB"},
		}

		stdout, _ := captureOutput(t, func() {
			Table(headers, rows)
		})

		assert.Contains(t, stdout, "Name")
		assert.Contains(t, stdout, "Size")
		assert.Contains(t, stdout, "package1")
		assert.Contains(t, stdout, "package2")
	})
}

func TestPrintKeyValue(t *testing.T) {
	t.Run("When called, it outputs key and value to stdout", func(t *testing.T) {
		stdout, _ := captureOutput(t, func() {
			PrintKeyValue("Key", "Value")
		})

		assert.Contains(t, stdout, "Key")
		assert.Contains(t, stdout, "Value")
	})
}

// captureOutput captures both stdout and stderr during fn execution
func captureOutput(t *testing.T, fn func()) (stdout string, stderr string) {
	t.Helper()

	// Capture stdout
	var stdoutBuf bytes.Buffer
	oldStdout := os.Stdout
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutW

	// Capture stderr (color library writes here by default)
	var stderrBuf bytes.Buffer
	oldStderr := os.Stderr
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = stderrW

	// The fatih/color library uses color.Output which defaults to os.Stderr.
	// We need to update it to point to our pipe.
	oldColorOutput := color.Output
	color.Output = stderrW

	fn()

	stdoutW.Close()
	stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	color.Output = oldColorOutput

	stdoutBuf.ReadFrom(stdoutR)
	stderrBuf.ReadFrom(stderrR)

	return stdoutBuf.String(), stderrBuf.String()
}
