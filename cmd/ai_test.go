package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/filemode"
	"github.com/stretchr/testify/require"
)

func resetAIUsageCLIState(t *testing.T) {
	t.Helper()
	require.NoError(t, rootCmd.PersistentFlags().Set("json", "false"))
	require.NoError(t, aiUsageCmd.Flags().Set("provider", ""))
	require.NoError(t, aiUsageCmd.Flags().Set("no-cache", "false"))
	require.NoError(t, aiTokensCmd.Flags().Set("provider", ""))
	require.NoError(t, aiTokensCmd.Flags().Set("since", ""))
	require.NoError(t, aiTokensCmd.Flags().Set("until", ""))
	require.NoError(t, aiTokensCmd.Flags().Set("group-by", "provider,model"))
	require.NoError(t, aiTokensCmd.Flags().Set("sort-by", ""))
	require.NoError(t, aiTokensCmd.Flags().Set("sort-order", ""))
	require.NoError(t, aiTokensCmd.Flags().Set("show-total-tokens", "false"))
}

func TestRunAIUsage_unknownProvider(t *testing.T) {
	resetAIUsageCLIState(t)
	defer func() { rootCmd.SetArgs([]string{}) }()
	rootCmd.SetArgs([]string{"ai", "usage", "--provider", "wat"})
	err := rootCmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown provider`)
}

func TestRunAIUsage_emptyProviderCSV(t *testing.T) {
	resetAIUsageCLIState(t)
	defer func() { rootCmd.SetArgs([]string{}) }()
	rootCmd.SetArgs([]string{"ai", "usage", "--provider", "  ,  , "})
	err := rootCmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--provider")
}

func TestRunAIUsage_jsonCacheHit(t *testing.T) {
	resetAIUsageCLIState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	cacheDir, err := ai.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(cacheDir, filemode.Dir))

	report := ai.AggregateReport{
		FetchedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Providers: []ai.ProviderResult{
			{Name: "claude", OK: true, Report: ai.UsageReport{
				Windows: []ai.UsageWindow{{Label: "5 hour", PercentUsed: 12}},
			}},
		},
	}
	payload := struct {
		ExpiresAt time.Time          `json:"expires_at"`
		Report    ai.AggregateReport `json:"report"`
	}{
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Report:    report,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "ai-usage.json"), b, 0o600))

	defer func() { rootCmd.SetArgs([]string{}) }()
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"ai", "usage", "-j"})
		err := rootCmd.Execute()
		require.NoError(t, err)
	})

	var decoded ai.AggregateReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Providers, 1)
	require.True(t, decoded.Providers[0].OK)
	require.Contains(t, out, `"name": "claude"`)
}

func TestRunAITokens_jsonLocalHistory(t *testing.T) {
	resetAIUsageCLIState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".codex", "sessions")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rollout.jsonl"), []byte(
		`{"type":"turn_context","timestamp":"2026-06-01T12:00:00Z","payload":{"model":"gpt-5-codex"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-06-01T12:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000000}}}}`+"\n",
	), 0o600))

	defer func() { rootCmd.SetArgs([]string{}) }()
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"ai", "tokens", "--provider", "codex", "-j"})
		err := rootCmd.Execute()
		require.NoError(t, err)
	})

	var decoded ai.HistoryReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Equal(t, "provider,model", decoded.GroupBy)
	require.Len(t, decoded.Groups, 1)
	require.Equal(t, "codex", decoded.Groups[0].Provider)
	require.Equal(t, "gpt-5-codex", decoded.Groups[0].Model)
	require.InDelta(t, 11.25, decoded.Total.CostUSD, 1e-9)
}

func TestRunAITokens_rejectsInvertedDateRange(t *testing.T) {
	resetAIUsageCLIState(t)
	defer func() { rootCmd.SetArgs([]string{}) }()
	rootCmd.SetArgs([]string{"ai", "tokens", "--since", "2026-06-04", "--until", "2026-06-03"})
	err := rootCmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--since must be on or before --until")
}

func TestRunAIUsage_printsROIWhenConfigured(t *testing.T) {
	resetAIUsageCLIState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	// The ROI section only counts sessions from the current month, so derive the
	// fixture timestamp from the live clock; back off an hour to stay behind the
	// command's own time.Now(), clamping to month start so it never crosses back.
	now := time.Now().UTC()
	sessionTime := now.Add(-time.Hour)
	if sessionTime.Month() != now.Month() {
		sessionTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	cacheDir, err := ai.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(cacheDir, filemode.Dir))
	report := ai.AggregateReport{
		FetchedAt: sessionTime,
		Providers: []ai.ProviderResult{
			{Name: "codex", OK: true, Report: ai.UsageReport{Windows: []ai.UsageWindow{{Label: "5 hour", PercentUsed: 1}}}},
		},
	}
	payload := struct {
		ExpiresAt time.Time          `json:"expires_at"`
		Report    ai.AggregateReport `json:"report"`
	}{
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Report:    report,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "ai-usage.json"), b, 0o600))

	cfgPath, err := ai.ConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), filemode.Dir))
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"subscriptions":{"codex":1}}`), 0o600))

	sessionDir := filepath.Join(home, ".codex", "sessions")
	require.NoError(t, os.MkdirAll(sessionDir, filemode.Dir))
	ts := sessionTime.Format(time.RFC3339)
	ts2 := sessionTime.Add(time.Second).Format(time.RFC3339)
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "rollout.jsonl"), []byte(
		`{"type":"turn_context","timestamp":"`+ts+`","payload":{"model":"gpt-5-codex"}}`+"\n"+
			`{"type":"event_msg","timestamp":"`+ts2+`","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000}}}}`+"\n",
	), 0o600))

	defer func() { rootCmd.SetArgs([]string{}) }()
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"ai", "usage"})
		err := rootCmd.Execute()
		require.NoError(t, err)
	})
	require.Contains(t, out, "Subscription ROI")
	require.Contains(t, out, "codex")
	require.Contains(t, out, "$1.25")
	require.Contains(t, out, "$1.00")
	require.Contains(t, out, "1.3x")
}
