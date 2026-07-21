package codex

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
	dir := filepath.Join(home, ".codex", "sessions", "2026", "06", "01")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	path := filepath.Join(dir, "rollout-2026-06-01T12-00-00-019db6d1-aaaa-bbbb.jsonl")
	body := `{"type":"session_meta","timestamp":"2026-06-01T12:00:00Z","payload":{"id":"019db6d1-aaaa-bbbb-cccc-dddddddddddd","cwd":"/home/u/proj","model_provider":"openai"}}
{"type":"turn_context","timestamp":"2026-06-01T12:00:00Z","payload":{"model":"gpt-5.4"}}
{"type":"response_item","timestamp":"2026-06-01T12:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n<cwd>/home/u/proj</cwd>\n</environment_context>"}]}}
{"type":"event_msg","timestamp":"2026-06-01T12:00:01Z","payload":{"type":"user_message","message":"help me refactor the parser"}}
{"type":"event_msg","timestamp":"2026-06-01T12:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"output_tokens":50},"total_token_usage":{"input_tokens":1000,"output_tokens":50}}}}
`
	require.NoError(t, os.WriteFile(path, []byte(body), filemode.File))

	sessions, err := (&HistoryProvider{HomeDir: home}).Sessions(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]
	require.Equal(t, "codex", s.Provider)
	require.Equal(t, "019db6d1-aaaa-bbbb-cccc-dddddddddddd", s.ResumeID) // from session_meta, not the filename
	require.Equal(t, "/home/u/proj", s.Project)
	require.Equal(t, "help me refactor the parser", s.Title) // environment_context turn is skipped
	require.Equal(t, "gpt-5.4", s.Model)
	require.Equal(t, 1, s.Messages)
	require.Greater(t, s.Tokens.Total(), int64(0))
}

func TestHistoryProvider_Sessions_emptyHomeReturnsNone(t *testing.T) {
	sessions, err := (&HistoryProvider{HomeDir: t.TempDir()}).Sessions(context.Background())
	require.NoError(t, err)
	require.Empty(t, sessions)
}
