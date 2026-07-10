package stats

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogPath_respectsXDGStateHome(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)

	path, err := LogPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "arc", "invocations.jsonl"), path)
}

func TestAppendReadAll_roundtrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	first := Entry{Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), Command: "update system", OK: true, DurationMS: 1500}
	second := Entry{Timestamp: time.Date(2026, 7, 2, 11, 0, 0, 0, time.UTC), Command: "clean", OK: false, DurationMS: 40}
	require.NoError(t, Append(first))
	require.NoError(t, Append(second))

	entries, err := ReadAll()
	require.NoError(t, err)
	require.Equal(t, []Entry{first, second}, entries)
}

func TestReadAll_missingFileReturnsNil(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	entries, err := ReadAll()
	require.NoError(t, err)
	require.Nil(t, entries)
}

func TestReadAll_skipsCorruptLines(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	valid := Entry{Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), Command: "packages", OK: true, DurationMS: 10}
	require.NoError(t, Append(valid))

	path, err := LogPath()
	require.NoError(t, err)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString("{torn wri\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, Append(valid))

	entries, err := ReadAll()
	require.NoError(t, err)
	require.Equal(t, []Entry{valid, valid}, entries)
}

func TestEnabled(t *testing.T) {
	t.Run("default is enabled", func(t *testing.T) {
		t.Setenv(NoTrackEnvVar, "")
		require.NoError(t, os.Unsetenv(NoTrackEnvVar))
		require.True(t, Enabled())
	})
	t.Run("truthy value disables", func(t *testing.T) {
		t.Setenv(NoTrackEnvVar, "1")
		require.False(t, Enabled())
		t.Setenv(NoTrackEnvVar, "true")
		require.False(t, Enabled())
	})
	t.Run("falsy or garbage stays enabled", func(t *testing.T) {
		t.Setenv(NoTrackEnvVar, "0")
		require.True(t, Enabled())
		t.Setenv(NoTrackEnvVar, "banana")
		require.True(t, Enabled())
	})
}

func TestAggregate_groupsAndSorts(t *testing.T) {
	early := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	report := Aggregate([]Entry{
		{Timestamp: late, Command: "update system", OK: true, DurationMS: 200},
		{Timestamp: early, Command: "update system", OK: false, DurationMS: 100},
		{Timestamp: early, Command: "clean", OK: true, DurationMS: 50},
		{Timestamp: early, Command: "ai usage", OK: true, DurationMS: 10},
	})

	require.Equal(t, 4, report.Total)
	require.Len(t, report.Commands, 3)

	top := report.Commands[0]
	require.Equal(t, "update system", top.Command)
	require.Equal(t, 2, top.Count)
	require.Equal(t, 1, top.Failures)
	require.Equal(t, early, top.FirstUsed)
	require.Equal(t, late, top.LastUsed)
	require.Equal(t, int64(300), top.TotalMS)

	// Ties on count break alphabetically for stable output.
	require.Equal(t, "ai usage", report.Commands[1].Command)
	require.Equal(t, "clean", report.Commands[2].Command)
}

func TestAggregate_empty(t *testing.T) {
	report := Aggregate(nil)
	require.Equal(t, 0, report.Total)
	require.Empty(t, report.Commands)
}
