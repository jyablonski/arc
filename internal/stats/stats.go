// Package stats records arc command invocations to a local append-only JSONL
// log and aggregates them for `arc stats`. Only the canonical command path,
// outcome, and duration are stored — never arguments, flag values, or any
// environment data — and nothing leaves the machine. Recording is best-effort:
// callers ignore failures so tracking can never break the command being run.
package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/filemode"
)

// NoTrackEnvVar disables invocation tracking when set to a truthy value.
const NoTrackEnvVar = "ARC_NO_TRACK"

const logFileName = "invocations.jsonl"

// Entry is one recorded invocation: a single line in the log file.
type Entry struct {
	Timestamp  time.Time `json:"ts"`
	Command    string    `json:"cmd"`
	OK         bool      `json:"ok"`
	DurationMS int64     `json:"ms"`
}

// Enabled reports whether tracking is on (the default). Only an explicit
// truthy ARC_NO_TRACK value turns it off.
func Enabled() bool {
	value, ok := os.LookupEnv(NoTrackEnvVar)
	if !ok {
		return true
	}
	disabled, err := strconv.ParseBool(strings.TrimSpace(value))
	return err != nil || !disabled
}

// LogPath is the invocation log location: $XDG_STATE_HOME/arc/invocations.jsonl,
// defaulting to ~/.local/state/arc on Linux. macOS has no state-dir convention,
// so it falls back to the user config dir (~/Library/Application Support/arc).
// State is deliberately kept out of the cache dir so cache wipes don't erase it.
func LogPath() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "arc", logFileName), nil
	}
	if runtime.GOOS == "darwin" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "arc", logFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "arc", logFileName), nil
}

// Append writes one entry to the log. Each entry is a single short JSON line
// written with O_APPEND, which POSIX appends atomically at this size, so
// concurrent arc invocations cannot interleave or lose records without any
// locking.
func Append(e Entry) error {
	path, err := LogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), filemode.Dir); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ReadAll parses the log, returning nil when it does not exist yet. Lines that
// fail to parse (e.g. a torn write from a crash) are skipped rather than
// failing the whole read.
func ReadAll() ([]Entry, error) {
	path, err := LogPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// CommandStats is the aggregate for one command path.
type CommandStats struct {
	Command   string    `json:"command"`
	Count     int       `json:"count"`
	Failures  int       `json:"failures"`
	FirstUsed time.Time `json:"first_used"`
	LastUsed  time.Time `json:"last_used"`
	TotalMS   int64     `json:"total_ms"`
}

// Report is the aggregated view rendered by `arc stats`.
type Report struct {
	Total    int            `json:"total"`
	Commands []CommandStats `json:"commands"`
}

// Aggregate groups entries by command, most-invoked first (ties break
// alphabetically so output is stable).
func Aggregate(entries []Entry) Report {
	byCmd := make(map[string]*CommandStats)
	for _, e := range entries {
		cs, ok := byCmd[e.Command]
		if !ok {
			cs = &CommandStats{Command: e.Command, FirstUsed: e.Timestamp, LastUsed: e.Timestamp}
			byCmd[e.Command] = cs
		}
		cs.Count++
		if !e.OK {
			cs.Failures++
		}
		if e.Timestamp.Before(cs.FirstUsed) {
			cs.FirstUsed = e.Timestamp
		}
		if e.Timestamp.After(cs.LastUsed) {
			cs.LastUsed = e.Timestamp
		}
		cs.TotalMS += e.DurationMS
	}

	report := Report{Total: len(entries), Commands: make([]CommandStats, 0, len(byCmd))}
	for _, cs := range byCmd {
		report.Commands = append(report.Commands, *cs)
	}
	sort.Slice(report.Commands, func(i, j int) bool {
		a, b := report.Commands[i], report.Commands[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Command < b.Command
	})
	return report
}
