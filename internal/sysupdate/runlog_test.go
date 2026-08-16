package sysupdate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewRunLog_privateAndDiscoverable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	t.Setenv(updateLogDirEnv, dir)
	log, err := newRunLog(time.Date(2026, 8, 13, 7, 51, 25, 0, time.UTC))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })

	require.Equal(t, dir, filepath.Dir(log.path))
	require.Contains(t, filepath.Base(log.path), "update-20260813-075125-")
	info, err := os.Stat(log.path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

func TestCleanUpdateLogs_removesOnlyGeneratedLogs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	t.Setenv(updateLogDirEnv, dir)

	first, err := newRunLog(time.Date(2026, 8, 13, 7, 51, 25, 0, time.UTC))
	require.NoError(t, err)
	_, err = first.file.WriteString("first log\n")
	require.NoError(t, err)
	require.NoError(t, first.Close())
	firstInfo, err := os.Stat(first.path)
	require.NoError(t, err)

	second, err := newRunLog(time.Date(2026, 8, 13, 8, 12, 3, 0, time.UTC))
	require.NoError(t, err)
	_, err = second.file.WriteString("second log\n")
	require.NoError(t, err)
	require.NoError(t, second.Close())
	secondInfo, err := os.Stat(second.path)
	require.NoError(t, err)

	unrelated := filepath.Join(dir, "invocations.jsonl")
	require.NoError(t, os.WriteFile(unrelated, []byte("keep\n"), 0o600))
	lookalike := filepath.Join(dir, "update-not-an-arc-log.log")
	require.NoError(t, os.WriteFile(lookalike, []byte("keep\n"), 0o600))

	result, err := CleanUpdateLogs()
	require.NoError(t, err)
	require.Equal(t, LogCleanupResult{Files: 2, Bytes: firstInfo.Size() + secondInfo.Size()}, result)
	require.NoFileExists(t, first.path)
	require.NoFileExists(t, second.path)
	require.FileExists(t, unrelated)
	require.FileExists(t, lookalike)
	require.DirExists(t, dir)
}

func TestCleanUpdateLogs_missingDirectoryIsNoOp(t *testing.T) {
	t.Setenv(updateLogDirEnv, filepath.Join(t.TempDir(), "missing"))

	result, err := CleanUpdateLogs()
	require.NoError(t, err)
	require.Equal(t, LogCleanupResult{}, result)
}

func TestUpdateLogDir_respectsXDGStateHome(t *testing.T) {
	base := t.TempDir()
	t.Setenv(updateLogDirEnv, "")
	t.Setenv("XDG_STATE_HOME", base)

	dir, err := updateLogDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "arc", "update-logs"), dir)
}

func TestCleanUpdateLogs_rejectsNonDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	t.Setenv(updateLogDirEnv, path)

	_, err := CleanUpdateLogs()
	require.ErrorContains(t, err, "read update log directory")
}

func TestIsUpdateLogName(t *testing.T) {
	require.True(t, isUpdateLogName("update-20260813-075125-123456789.log"))
	for _, name := range []string{
		"update-20260813-075125-.log",
		"update-20260813-075125-random.log",
		"update-20269999-075125-123.log",
		"update-not-an-arc-log.log",
		"invocations.jsonl",
	} {
		require.False(t, isUpdateLogName(name), name)
	}
}

func TestRunLogTail_boundedAndSanitized(t *testing.T) {
	t.Setenv(updateLogDirEnv, t.TempDir())
	log, err := newRunLog(time.Now())
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })

	for i := 0; i < maxTailLines+10; i++ {
		_, err = fmt.Fprintf(log.file, "line %02d\n", i)
		require.NoError(t, err)
	}
	_, err = log.file.WriteString("\x1b[31mERROR\x1b[0m \x1b]0;hostile title\aunsafe\x00text\rprogress\n")
	require.NoError(t, err)

	tail := log.tail()
	require.Len(t, tail, maxTailLines)
	require.Equal(t, "line 12", tail[0])
	require.Equal(t, "ERROR unsafetext", tail[len(tail)-2])
	require.Equal(t, "progress", tail[len(tail)-1])
	require.NotContains(t, strings.Join(tail, "\n"), "\x1b")
}

func TestRunLogTailFrom_excludesEarlierCommands(t *testing.T) {
	t.Setenv(updateLogDirEnv, t.TempDir())
	log, err := newRunLog(time.Now())
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })

	log.note("successful earlier command output")
	start := log.command("failing-command")
	_, err = log.file.WriteString("relevant failure\n")
	require.NoError(t, err)

	tail := log.tailFrom(start)
	require.Equal(t, []string{"relevant failure"}, tail)
}

func TestMemoryRunLogTailFrom_isBoundedAndCommandScoped(t *testing.T) {
	log := newMemoryRunLog()
	log.note("successful earlier command output")
	start := log.command("failing-command")
	_, err := io.WriteString(log.Writer(), strings.Repeat("x", maxTailBytes)+"\nrelevant failure\n")
	require.NoError(t, err)

	tail := log.tailFrom(start)
	require.Equal(t, []string{"relevant failure"}, tail)
	require.Empty(t, log.path)
}

func TestNewRunLog_creationFailure(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "not-a-directory")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	t.Setenv(updateLogDirEnv, file)

	_, err := newRunLog(time.Now())
	require.ErrorContains(t, err, "create update log directory")
}
