package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestHistoryProvider_LocalUsage_parsesClaudeJSONLAndDedups(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-repo")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	path := filepath.Join(dir, "session-1.jsonl")
	body := `{"type":"assistant","timestamp":"2026-06-01T12:00:00Z","requestId":"r1","message":{"id":"m1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":30}}}
{"type":"assistant","timestamp":"2026-06-01T12:00:01Z","requestId":"r1","message":{"id":"m1","model":"claude-sonnet-4-20250514","usage":{"input_tokens":120,"output_tokens":25,"cache_read_input_tokens":30,"cache_creation_input_tokens":5}}}
{"type":"user","timestamp":"2026-06-01T12:00:02Z","message":{"content":"ignored"}}
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	p := &HistoryProvider{HomeDir: home}
	records, err := p.LocalUsage(context.Background(), ai.HistoryOptions{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "claude", records[0].Provider)
	require.Equal(t, "claude-sonnet-4-20250514", records[0].Model)
	require.Equal(t, int64(120), records[0].Tokens.Input)
	require.Equal(t, int64(25), records[0].Tokens.Output)
	require.Equal(t, int64(30), records[0].Tokens.CacheRead)
	require.Equal(t, int64(5), records[0].Tokens.CacheWrite)
}

func TestHistoryProvider_LocalUsage_sinceFilter(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-repo")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "session-1.jsonl"), []byte(
		`{"type":"assistant","timestamp":"2026-05-01T12:00:00Z","message":{"id":"m1","model":"claude-sonnet-4","usage":{"input_tokens":100}}}`+"\n",
	), 0o600))

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	records, err := (&HistoryProvider{HomeDir: home}).LocalUsage(context.Background(), ai.HistoryOptions{Since: &since})
	require.NoError(t, err)
	require.Empty(t, records)
}
