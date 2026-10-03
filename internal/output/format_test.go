package output

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepHelpers_writeSharedGlyphsToStdout(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	stdout, stderr := captureOutput(t, func() {
		Success("done")
		Error("broke")
		Info("fyi")
		Warning("careful")
	})
	assert.Equal(t, "✓ done\n✗ broke\n· fyi\n⚠ careful\n", stdout)
	assert.Empty(t, stderr)
}

func TestTitleSectionSummary_writeLiveGrammarToStdout(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("COLUMNS", "")
	stdout, _ := captureOutput(t, func() {
		Title("arc clean", "linux")
		Section("package cache")
		Success("cache cleaned")
		Summary(GlyphOK, "clean complete", "", "2 steps")
	})
	lines := strings.Split(stdout, "\n")
	require.Len(t, lines, 8)
	assert.Equal(t, "arc clean"+strings.Repeat(" ", FrameWidth-len("arc clean")-len("linux"))+"linux", lines[0])
	assert.Equal(t, strings.Repeat("─", FrameWidth), lines[1])
	assert.Empty(t, lines[2])
	assert.Equal(t, "package cache", lines[3])
	assert.Equal(t, "✓ cache cleaned", lines[4])
	assert.Empty(t, lines[5])
	// Empty fragments are dropped rather than leaving a doubled separator.
	assert.Equal(t, "✓ clean complete · 2 steps", lines[6])
}

func TestSummary_downgradesAnOKVerdictAfterProblemSteps(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	stdout, _ := captureOutput(t, func() {
		Title("arc setup", "")
		Success("installed gh")
		Warning("lshw not installed: exit status 1")
		Error("uv not installed: exit status 1")
		Summary(GlyphOK, "setup complete")

		// A new title starts a clean tally, and a verdict that is already a
		// warning keeps its own wording.
		Title("arc clean", "")
		Summary(GlyphOK, "clean complete")
		Title("arc docker clean", "")
		Warning("images not pruned")
		Summary(GlyphWarn, "1 of 3 prunes failed")
	})
	assert.Contains(t, stdout, "\n\n⚠ setup complete · 2 warnings\n")
	// No body: the verdict sits directly under the title's blank line.
	assert.Contains(t, stdout, strings.Repeat("─", FrameWidth)+"\n\n✓ clean complete\n")
	assert.Contains(t, stdout, "\n\n⚠ 1 of 3 prunes failed\n")
}

func TestFailure_writesToStderrAfterAGap(t *testing.T) {
	t.Setenv("ARC_ASCII", "")
	t.Setenv("LANG", "en_US.UTF-8")
	stdout, stderr := captureOutput(t, func() {
		Title("arc clean", "")
		Failure(errors.New("pacman is not available"))
		Info("cleaning")
		Failure(errors.New("failed to clean cache"))
	})
	assert.NotContains(t, stdout, "pacman is not available")
	assert.Equal(t, "✗ pacman is not available\n\n✗ failed to clean cache\n", stderr)
}

func TestBytes(t *testing.T) {
	require.Equal(t, "0 B", Bytes(0))
	require.Equal(t, "1.5 KiB", Bytes(1536))
	require.Equal(t, "2.0 MiB", Bytes(2*1024*1024))
	require.Equal(t, "1.0 GiB", Bytes(1024*1024*1024))
}

func captureOutput(t *testing.T, fn func()) (stdout string, stderr string) {
	t.Helper()

	// Capture stdout
	var stdoutBuf bytes.Buffer
	oldStdout := os.Stdout
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutW

	// Capture stderr
	var stderrBuf bytes.Buffer
	oldStderr := os.Stderr
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = stderrW

	fn()

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	_, err = stdoutBuf.ReadFrom(stdoutR)
	require.NoError(t, err)
	_, err = stderrBuf.ReadFrom(stderrR)
	require.NoError(t, err)

	return stdoutBuf.String(), stderrBuf.String()
}
