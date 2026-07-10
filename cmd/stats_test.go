package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/stats"
	"github.com/stretchr/testify/require"
)

func TestStatsCmd_emptyLog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	defer func() { rootCmd.SetArgs(nil) }()

	// output.Info writes through the color package (bound to the real stdout at
	// init), so captureStdout can't see the empty-log message; assert that no
	// table was rendered instead.
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		require.NoError(t, rootCmd.Execute())
	})
	require.NotContains(t, out, "command")
}

func TestStatsCmd_printsTable(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ts := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	require.NoError(t, stats.Append(stats.Entry{Timestamp: ts, Command: "update system", OK: true, DurationMS: 2000}))
	require.NoError(t, stats.Append(stats.Entry{Timestamp: ts, Command: "update system", OK: false, DurationMS: 1000}))
	require.NoError(t, stats.Append(stats.Entry{Timestamp: ts, Command: "clean", OK: true, DurationMS: 100}))
	defer func() { rootCmd.SetArgs(nil) }()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		require.NoError(t, rootCmd.Execute())
	})
	require.Contains(t, out, "command")
	require.Contains(t, out, "failures")
	require.Contains(t, out, "update system")
	require.Contains(t, out, "clean")
	require.Contains(t, out, "3s")
	require.Contains(t, out, "100ms")
}

func TestStatsCmd_json(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ts := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	require.NoError(t, stats.Append(stats.Entry{Timestamp: ts, Command: "packages", OK: true, DurationMS: 42}))
	defer func() { rootCmd.SetArgs(nil) }()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats", "-j"})
		require.NoError(t, rootCmd.Execute())
	})

	var report stats.Report
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	require.Equal(t, 1, report.Total)
	require.Len(t, report.Commands, 1)
	require.Equal(t, "packages", report.Commands[0].Command)
	require.Equal(t, int64(42), report.Commands[0].TotalMS)
}

func TestRecordInvocation_writesTrackedCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	c, _, err := rootCmd.Find([]string{"update", "system"})
	require.NoError(t, err)
	recordInvocation(c, false, 1500*time.Millisecond)

	entries, err := stats.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "update system", entries[0].Command)
	require.False(t, entries[0].OK)
	require.Equal(t, int64(1500), entries[0].DurationMS)
}

func TestRecordInvocation_skipsExcludedAndDisabled(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Root, help, and stats itself are never tracked.
	recordInvocation(rootCmd, true, time.Second)
	recordInvocation(statsCmd, true, time.Second)
	helpCmd, _, err := rootCmd.Find([]string{"help"})
	require.NoError(t, err)
	recordInvocation(helpCmd, true, time.Second)

	// ARC_NO_TRACK disables tracking of an otherwise tracked command.
	t.Setenv(stats.NoTrackEnvVar, "1")
	c, _, err := rootCmd.Find([]string{"clean"})
	require.NoError(t, err)
	recordInvocation(c, true, time.Second)

	entries, err := stats.ReadAll()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestTrackable(t *testing.T) {
	require.True(t, trackable("update system"))
	require.True(t, trackable("clean"))
	require.False(t, trackable(""))
	require.False(t, trackable("help"))
	require.False(t, trackable("completion bash"))
	require.False(t, trackable("__complete"))
	require.False(t, trackable("stats"))
}
