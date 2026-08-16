package output

import (
	"bytes"
	"os"
	"strings"
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

func TestSectionAccent(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	stdout, _ := captureOutput(t, func() {
		ac := color.New(color.FgGreen)
		SectionAccent("Codex", ac)
	})
	assert.Contains(t, stdout, "Codex")
	assert.Contains(t, stdout, "─────")
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

func TestBytes(t *testing.T) {
	require.Equal(t, "0 B", Bytes(0))
	require.Equal(t, "1.5 KiB", Bytes(1536))
	require.Equal(t, "2.0 MiB", Bytes(2*1024*1024))
	require.Equal(t, "1.0 GiB", Bytes(1024*1024*1024))
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

		// Headers are lowercased into the canonical format.
		assert.Contains(t, stdout, "name")
		assert.Contains(t, stdout, "size")
		assert.NotContains(t, stdout, "Name")
		assert.Contains(t, stdout, "package1")
		assert.Contains(t, stdout, "package2")
	})
}

func TestTableLines(t *testing.T) {
	t.Run("lowercases headers and underlines each column", func(t *testing.T) {
		lines := TableLines(
			[]string{"NAME", "STATUS"},
			[][]string{{"demo", "ok"}},
		)
		require.Len(t, lines, 3)
		assert.Equal(t, "name  status", lines[0])
		assert.Equal(t, "----  ------", lines[1])
		assert.Equal(t, "demo  ok", lines[2])
	})

	t.Run("aligns cells by visible width ignoring ANSI color codes", func(t *testing.T) {
		colored := "\x1b[38;2;255;133;31mclaude/opus\x1b[0m" // 11 visible runes
		lines := TableLines(
			[]string{"group", "share"},
			[][]string{
				{colored, "64.5%"},
				{"codex/gpt-5", "35.5%"},
			},
		)
		// Both group cells have 11 visible runes, so the share column must
		// start at the same offset on both data rows.
		plain := ansiPattern.ReplaceAllString(lines[2], "")
		require.Equal(t, strings.Index(plain, "64.5%"), strings.Index(lines[3], "35.5%"))
	})

	t.Run("trims trailing whitespace from padded cells", func(t *testing.T) {
		lines := TableLines(
			[]string{"group", "api equiv"},
			[][]string{
				{"codex", "$3.47  "},
				{"total", "$10.79"},
			},
		)
		for _, line := range lines {
			assert.False(t, strings.HasSuffix(line, " "), line)
		}
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

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	color.Output = oldColorOutput

	_, err = stdoutBuf.ReadFrom(stdoutR)
	require.NoError(t, err)
	_, err = stderrBuf.ReadFrom(stderrR)
	require.NoError(t, err)

	return stdoutBuf.String(), stderrBuf.String()
}
