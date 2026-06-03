package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func TestHistoryProvider_LocalUsage_parsesTokenCountLastUsage(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "06", "01")
	require.NoError(t, os.MkdirAll(dir, filemode.Dir))
	path := filepath.Join(dir, "rollout-1.jsonl")
	body := `{"type":"session_meta","payload":{"model_provider":"openai"}}
{"type":"turn_context","timestamp":"2026-06-01T12:00:00Z","payload":{"model":"gpt-5-codex"}}
{"type":"event_msg","timestamp":"2026-06-01T12:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":50,"reasoning_output_tokens":25},"total_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":50,"reasoning_output_tokens":25}}}}
{"type":"event_msg","timestamp":"2026-06-01T12:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":200,"cached_input_tokens":50,"output_tokens":10},"total_token_usage":{"input_tokens":1200,"cached_input_tokens":450,"output_tokens":60,"reasoning_output_tokens":25}}}}
`
	require.NoError(t, os.WriteFile(path, []byte(body), filemode.File))

	records, err := (&HistoryProvider{HomeDir: home}).LocalUsage(context.Background(), ai.HistoryOptions{})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "codex", records[0].Provider)
	require.Equal(t, "gpt-5-codex", records[0].Model)
	require.Equal(t, int64(600), records[0].Tokens.Input)
	require.Equal(t, int64(400), records[0].Tokens.CacheRead)
	require.Equal(t, int64(50), records[0].Tokens.Output)
	require.Equal(t, int64(25), records[0].Tokens.Reasoning)
	require.Equal(t, int64(150), records[1].Tokens.Input)
	require.Equal(t, int64(50), records[1].Tokens.CacheRead)
	require.Equal(t, int64(10), records[1].Tokens.Output)
}
