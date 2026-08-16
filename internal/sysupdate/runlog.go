package sysupdate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/statepath"
)

const (
	updateLogDirEnv = "ARC_UPDATE_LOG_DIR"
	maxTailBytes    = 16 << 10
	maxTailLines    = 40
)

type runLog struct {
	file   *os.File
	memory *tailBuffer
	writer io.Writer
	path   string
}

type tailBuffer struct {
	data    []byte
	written int64
}

// LogCleanupResult summarizes update logs removed by CleanUpdateLogs.
type LogCleanupResult struct {
	Files int
	Bytes int64
}

func newRunLog(now time.Time) (*runLog, error) {
	dir, err := updateLogDir()
	if err != nil {
		return nil, err
	}
	return newRunLogIn(dir, now)
}

func newRunLogIn(dir string, now time.Time) (*runLog, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create update log directory: %w", err)
	}
	prefix := "update-" + now.Format("20060102-150405") + "-"
	f, err := os.CreateTemp(dir, prefix+"*.log")
	if err != nil {
		return nil, fmt.Errorf("create update log: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("protect update log: %w", err)
	}
	return &runLog{file: f, writer: f, path: f.Name()}, nil
}

func newMemoryRunLog() *runLog {
	memory := &tailBuffer{}
	return &runLog{memory: memory, writer: memory}
}

func updateLogDir() (string, error) {
	if dir := os.Getenv(updateLogDirEnv); dir != "" {
		return dir, nil
	}
	dir, err := statepath.ArcDir()
	if err != nil {
		return "", fmt.Errorf("resolve state directory for update logs: %w", err)
	}
	return filepath.Join(dir, "update-logs"), nil
}

// CleanUpdateLogs removes only logs created by arc update system. The log
// directory and unrelated files are preserved, including when the directory is
// overridden to a shared location with ARC_UPDATE_LOG_DIR.
func CleanUpdateLogs() (LogCleanupResult, error) {
	dir, err := updateLogDir()
	if err != nil {
		return LogCleanupResult{}, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return LogCleanupResult{}, nil
	}
	if err != nil {
		return LogCleanupResult{}, fmt.Errorf("read update log directory: %w", err)
	}

	var result LogCleanupResult
	for _, entry := range entries {
		if !isUpdateLogName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return result, fmt.Errorf("inspect update log %q: %w", filepath.Join(dir, entry.Name()), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			return result, fmt.Errorf("remove update log %q: %w", path, err)
		}
		result.Files++
		result.Bytes += info.Size()
	}
	return result, nil
}

func isUpdateLogName(name string) bool {
	const (
		prefix       = "update-"
		suffix       = ".log"
		timestampLen = len("20060102-150405")
	)
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if len(stem) <= timestampLen+1 || stem[timestampLen] != '-' {
		return false
	}
	if _, err := time.Parse("20060102-150405", stem[:timestampLen]); err != nil {
		return false
	}
	for _, r := range stem[timestampLen+1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (l *runLog) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *runLog) Writer() io.Writer {
	if l == nil || l.writer == nil {
		return io.Discard
	}
	return l.writer
}

func (l *runLog) command(name string, args ...string) int64 {
	if l == nil {
		return 0
	}
	parts := append([]string{name}, args...)
	for i, part := range parts {
		parts[i] = strconv.Quote(part)
	}
	_, _ = fmt.Fprintf(l.Writer(), "\n[%s] command %s\n", time.Now().Format(time.RFC3339), strings.Join(parts, " "))
	return l.position()
}

func (l *runLog) note(message string) {
	if l == nil {
		return
	}
	_, _ = fmt.Fprintf(l.Writer(), "[%s] %s\n", time.Now().Format(time.RFC3339), message)
}

func (l *runLog) tail() []string {
	return l.tailFrom(0)
}

func (l *runLog) position() int64 {
	if l == nil {
		return 0
	}
	if l.memory != nil {
		return l.memory.written
	}
	if l.file == nil {
		return 0
	}
	position, err := l.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}
	return position
}

func (l *runLog) tailFrom(commandStart int64) []string {
	if l == nil {
		return nil
	}
	if l.memory != nil {
		b, truncated := l.memory.from(commandStart)
		return sanitizedTail(b, truncated)
	}
	if l.file == nil {
		return nil
	}
	_ = l.file.Sync()
	stat, err := l.file.Stat()
	if err != nil {
		return nil
	}
	commandStart = min(max(int64(0), commandStart), stat.Size())
	start := max(commandStart, stat.Size()-maxTailBytes)
	truncated := start > commandStart
	if _, err := l.file.Seek(start, io.SeekStart); err != nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(l.file, maxTailBytes))
	_, _ = l.file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil
	}
	return sanitizedTail(b, truncated)
}

func sanitizedTail(b []byte, truncated bool) []string {
	if truncated {
		if newline := bytes.IndexByte(b, '\n'); newline >= 0 {
			b = b[newline+1:]
		}
	}
	text := strings.ReplaceAll(string(b), "\r", "\n")
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(sanitizeTerminal(scanner.Text()))
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > maxTailLines {
		lines = lines[len(lines)-maxTailLines:]
	}
	return lines
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	b.written += int64(n)
	if n >= maxTailBytes {
		b.data = append(b.data[:0], p[n-maxTailBytes:]...)
		return n, nil
	}
	b.data = append(b.data, p...)
	if excess := len(b.data) - maxTailBytes; excess > 0 {
		copy(b.data, b.data[excess:])
		b.data = b.data[:len(b.data)-excess]
	}
	return n, nil
}

func (b *tailBuffer) from(start int64) ([]byte, bool) {
	availableStart := b.written - int64(len(b.data))
	effectiveStart := min(max(start, availableStart), b.written)
	offset := effectiveStart - availableStart
	return append([]byte(nil), b.data[offset:]...), effectiveStart > start
}

func sanitizeTerminal(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '[':
				i++
				for i < len(s) {
					b := s[i]
					i++
					if b >= 0x40 && b <= 0x7e {
						break
					}
				}
			case ']':
				i++
				for i < len(s) {
					if s[i] == '\a' {
						i++
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			default:
				i++
			}
			continue
		}
		if s[i] < 0x20 && s[i] != '\t' {
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func closeRunLog(log *runLog, prior error) error {
	if log == nil {
		return prior
	}
	if err := log.Close(); err != nil && prior == nil {
		return fmt.Errorf("close update log: %w", err)
	} else if err != nil {
		return errors.Join(prior, fmt.Errorf("close update log: %w", err))
	}
	return prior
}
