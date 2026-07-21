package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestHistoryProvider_Sessions_parsesMetadataAndFoldsTokens(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-repo")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	body := `{"type":"user","cwd":"/home/u/proj","gitBranch":"main","timestamp":"2026-06-01T12:00:00Z","message":{"role":"user","content":"first real prompt"}}
{"type":"assistant","timestamp":"2026-06-01T12:00:05Z","requestId":"r1","message":{"id":"m1","model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":20}}}
{"type":"user","timestamp":"2026-06-01T12:00:06Z","message":{"role":"user","content":[{"type":"tool_result"}]}}
{"type":"assistant","timestamp":"2026-06-01T12:00:10Z","requestId":"r2","message":{"id":"m2","model":"claude-opus-4-8","usage":{"input_tokens":50,"output_tokens":10}}}
{"type":"ai-title","aiTitle":"My Session Title","sessionId":"session-1"}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "session-1.jsonl"), []byte(body), filemode.File))

	sessions, err := (&HistoryProvider{HomeDir: home}).Sessions(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]
	require.Equal(t, "claude", s.Provider)
	require.Equal(t, "session-1", s.SessionID)
	require.Equal(t, "session-1", s.ResumeID)
	require.Equal(t, "/home/u/proj", s.Project)
	require.Equal(t, "My Session Title", s.Title) // ai-title wins over the first prompt
	require.Equal(t, "claude-opus-4-8", s.Model)
	require.Equal(t, 2, s.Messages)
	require.Equal(t, int64(180), s.Tokens.Total())
	require.Equal(t, "2026-06-01T12:00:05Z", s.StartedAt.UTC().Format("2006-01-02T15:04:05Z"))
	require.Equal(t, "2026-06-01T12:00:10Z", s.LastAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func TestHistoryProvider_Sessions_titleFallsBackToFirstPrompt(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-repo")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	body := `{"type":"user","cwd":"/home/u/proj","timestamp":"2026-06-01T12:00:00Z","message":{"role":"user","content":"do the thing\nsecond line"}}
{"type":"assistant","timestamp":"2026-06-01T12:00:05Z","requestId":"r1","message":{"id":"m1","model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":20}}}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "session-2.jsonl"), []byte(body), filemode.File))

	sessions, err := (&HistoryProvider{HomeDir: home}).Sessions(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, "do the thing", sessions[0].Title) // first line of the first prompt
}

func TestHistoryProvider_Sessions_emptyHomeReturnsNone(t *testing.T) {
	sessions, err := (&HistoryProvider{HomeDir: t.TempDir()}).Sessions(context.Background())
	require.NoError(t, err)
	require.Empty(t, sessions)
}
