package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	aiclaude "github.com/jyablonski/arc/internal/ai/claude"
	aicodex "github.com/jyablonski/arc/internal/ai/codex"
	aicursor "github.com/jyablonski/arc/internal/ai/cursor"
	"github.com/jyablonski/arc/internal/ai/presentation"
	"github.com/spf13/cobra"
)

var (
	aiUsageProvider string
	aiUsageNoCache  bool
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI coding tool helpers",
	Long:  `Commands for working with Claude Code, Codex, Cursor, and related tooling.`,
}

var aiUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage limits across Claude, Codex, and Cursor",
	Long: `Fetches subscription or plan usage from each provider's local auth and
public or app-server APIs. Failures are isolated per provider.

Credentials are read fresh each run and are never written to the cache
(~/.cache/arc/ai-usage.json stores only aggregated numbers).

Use --provider to fetch one provider. Cache is skipped when --provider is set.`,
	RunE: runAIUsage,
}

func runAIUsage(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters := ai.ParseProviderCSV(aiUsageProvider)
	if aiUsageProvider != "" && len(filters) == 0 {
		return fmt.Errorf("--provider: empty after parsing")
	}
	if err := ai.ValidateProviderFilters(filters); err != nil {
		return err
	}

	providers := []ai.Provider{
		&aiclaude.Provider{},
		&aicodex.Provider{},
		&aicursor.Provider{},
	}

	useCache := !aiUsageNoCache && aiUsageProvider == ""
	now := time.Now()
	if useCache {
		if cached, ok, err := ai.ReadCache(now); err == nil && ok {
			if jsonOut {
				return aiExitJSON(os.Stdout, cached)
			}
			presentation.PrintAggregate(cached)
			return ai.ExitErrorIfAllProvidersFailed(cached)
		}
	}

	agg := ai.RunProviders(cmd.Context(), providers, filters)
	agg.FetchedAt = now

	if useCache {
		_ = ai.WriteCache(now, agg)
	}

	if jsonOut {
		return aiExitJSON(os.Stdout, agg)
	}
	presentation.PrintAggregate(agg)
	return ai.ExitErrorIfAllProvidersFailed(agg)
}

func aiExitJSON(w io.Writer, agg ai.AggregateReport) error {
	if err := ai.EncodeAggregateJSON(w, agg); err != nil {
		return err
	}
	return ai.ExitErrorIfAllProvidersFailed(agg)
}

func init() {
	aiUsageCmd.Flags().StringVar(&aiUsageProvider, "provider", "", "Only these providers (comma-separated): claude, codex, cursor")
	aiUsageCmd.Flags().BoolVar(&aiUsageNoCache, "no-cache", false, "Bypass ~/.cache/arc/ai-usage.json")
	aiCmd.AddCommand(aiUsageCmd)
	rootCmd.AddCommand(aiCmd)
}
