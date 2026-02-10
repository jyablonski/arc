package output

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeader(t *testing.T) {
	assert.NotPanics(t, func() {
		Header("Test Header")
	})
}

func TestSuccess(t *testing.T) {
	assert.NotPanics(t, func() {
		Success("Test success")
	})
}

func TestError(t *testing.T) {
	assert.NotPanics(t, func() {
		Error("Test error")
	})
}

func TestInfo(t *testing.T) {
	assert.NotPanics(t, func() {
		Info("Test info")
	})
}

func TestWarning(t *testing.T) {
	assert.NotPanics(t, func() {
		Warning("Test warning")
	})
}

func TestPrint(t *testing.T) {
	output := captureStdout(t, func() {
		Print("Test print")
	})

	assert.Contains(t, output, "Test print")
}

func TestTable(t *testing.T) {
	headers := []string{"Name", "Size"}
	rows := [][]string{
		{"package1", "100 MiB"},
		{"package2", "200 MiB"},
	}

	output := captureStdout(t, func() {
		Table(headers, rows)
	})

	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Size")
	assert.Contains(t, output, "package1")
	assert.Contains(t, output, "package2")
}

func TestPrintKeyValue(t *testing.T) {
	output := captureStdout(t, func() {
		PrintKeyValue("Key", "Value")
	})

	assert.Contains(t, output, "Key")
	assert.Contains(t, output, "Value")
}

// captureStdout is a test helper that captures stdout during fn execution
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)

	return buf.String()
}
