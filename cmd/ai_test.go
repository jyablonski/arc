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
	require.NoError(t, aiUsageCmd.Flags().Set("provider", ""))
	require.NoError(t, aiUsageCmd.Flags().Set("no-cache", "false"))
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
