package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
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

	filters := parseProviderCSV(aiUsageProvider)
	if aiUsageProvider != "" && len(filters) == 0 {
		return fmt.Errorf("--provider: empty after parsing")
	}
	if err := validateProviderFilters(filters); err != nil {
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
				return encodeAIUsageJSON(os.Stdout, cached)
			}
			presentation.PrintAggregate(cached)
			return exitCodeForAIUsage(cached)
		}
	}

	agg := ai.RunProviders(cmd.Context(), providers, filters)
	agg.FetchedAt = now

	if useCache {
		_ = ai.WriteCache(now, agg)
	}

	if jsonOut {
		if err := encodeAIUsageJSON(os.Stdout, agg); err != nil {
			return err
		}
		return exitCodeForAIUsage(agg)
	}
	presentation.PrintAggregate(agg)
	return exitCodeForAIUsage(agg)
}

func parseProviderCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateProviderFilters(filters []string) error {
	known := map[string]struct{}{"claude": {}, "codex": {}, "cursor": {}}
	for _, f := range filters {
		if _, ok := known[f]; !ok {
			return fmt.Errorf("unknown provider %q (claude, codex, cursor)", f)
		}
	}
	return nil
}

func encodeAIUsageJSON(w io.Writer, agg ai.AggregateReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(agg)
}

func exitCodeForAIUsage(agg ai.AggregateReport) error {
	if len(agg.Providers) == 0 {
		return fmt.Errorf("no providers selected")
	}
	anyOK := false
	for _, p := range agg.Providers {
		if p.OK {
			anyOK = true
			break
		}
	}
	if !anyOK {
		return ai.CombineErrors(agg)
	}
	return nil
}

func init() {
	aiUsageCmd.Flags().StringVar(&aiUsageProvider, "provider", "", "Only these providers (comma-separated): claude, codex, cursor")
	aiUsageCmd.Flags().BoolVar(&aiUsageNoCache, "no-cache", false, "Bypass ~/.cache/arc/ai-usage.json")
	aiCmd.AddCommand(aiUsageCmd)
	rootCmd.AddCommand(aiCmd)
}
