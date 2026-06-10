package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	aiclaude "github.com/jyablonski/arc/internal/ai/claude"
	aicodex "github.com/jyablonski/arc/internal/ai/codex"
	aicursor "github.com/jyablonski/arc/internal/ai/cursor"
	"github.com/jyablonski/arc/internal/ai/presentation"
	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/spf13/cobra"
)

var (
	aiUsageProvider         string
	aiUsageNoCache          bool
	aiTokensProvider        string
	aiTokensSince           string
	aiTokensUntil           string
	aiTokensGroupBy         string
	aiTokensSortBy          string
	aiTokensSortOrder       string
	aiTokensShowTotalTokens bool
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

var aiTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Show historical local AI token usage and API-equivalent cost",
	Long: `Scans local Claude Code and Codex CLI session logs and reports token usage
with an API-equivalent cost estimate based on built-in model pricing.

This is historical local usage, not the same thing as subscription quota usage.
Use "arc ai usage" for live provider-reported quota windows.`,
	RunE: runAITokens,
}

func runAIUsage(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters := ai.ParseProviderCSV(aiUsageProvider)
	if aiUsageProvider != "" && len(filters) == 0 {
		return arcerrs.ErrEmptyProviderFilter
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
			printUsageROI(cmd.Context(), filters, now)
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
	printUsageROI(cmd.Context(), filters, now)
	return ai.ExitErrorIfAllProvidersFailed(agg)
}

func runAITokens(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters := ai.ParseProviderCSV(aiTokensProvider)
	if aiTokensProvider != "" && len(filters) == 0 {
		return arcerrs.ErrEmptyProviderFilter
	}
	if err := ai.ValidateHistoryProviderFilters(filters); err != nil {
		return err
	}
	if err := ai.ValidateHistoryGroupBy(aiTokensGroupBy); err != nil {
		return err
	}
	if err := ai.ValidateHistorySort(aiTokensSortBy, aiTokensSortOrder); err != nil {
		return err
	}

	opts, err := parseHistoryOptions(aiTokensSince, aiTokensUntil)
	if err != nil {
		return err
	}
	report := ai.RunHistoryProviders(cmd.Context(), localHistoryProviders(), filters, opts, ai.NewStaticPricer(), aiTokensGroupBy)
	sortBy, sortOrder := ai.NormalizeHistorySort(report.GroupBy, aiTokensSortBy, aiTokensSortOrder)
	ai.SortUsageGroups(report.Groups, report.GroupBy, sortBy, sortOrder)
	report.SortBy = sortBy
	report.SortOrder = sortOrder

	if jsonOut {
		if err := ai.EncodeHistoryJSON(os.Stdout, report); err != nil {
			return err
		}
		return ai.ExitErrorIfAllHistoryProvidersFailed(report)
	}
	presentation.PrintHistory(report, presentation.HistoryPrintOptions{ShowTotalTokens: aiTokensShowTotalTokens})
	return ai.ExitErrorIfAllHistoryProvidersFailed(report)
}

func parseHistoryOptions(sinceRaw, untilRaw string) (ai.HistoryOptions, error) {
	var opts ai.HistoryOptions
	if sinceRaw != "" {
		t, err := parseHistoryDateFlag(sinceRaw)
		if err != nil {
			return opts, fmt.Errorf("--since: %w", err)
		}
		opts.Since = &t
	}
	if untilRaw != "" {
		t, err := parseHistoryDateFlag(untilRaw)
		if err != nil {
			return opts, fmt.Errorf("--until: %w", err)
		}
		opts.Until = &t
	}
	if opts.Since != nil && opts.Until != nil && opts.Since.After(*opts.Until) {
		return opts, fmt.Errorf("--since must be on or before --until")
	}
	return opts, nil
}

func parseHistoryDateFlag(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or RFC3339")
	}
	return t, nil
}

func printUsageROI(ctx context.Context, filters []string, now time.Time) {
	cfg, ok, err := ai.ReadConfig()
	if err != nil || !ok || !cfg.HasSubscriptions() {
		return
	}
	report := ai.RunHistoryProviders(ctx, localHistoryProviders(), filters, ai.CurrentMonthOptions(now), ai.NewStaticPricer(), "provider")
	byProvider := map[string]float64{}
	for _, group := range report.Groups {
		byProvider[group.Provider] = group.CostUSD
	}
	var summary presentation.ROISummary
	start := reportPeriodStart(now)
	summary.WindowLabel = fmt.Sprintf("%s to %s", start.Format("2006-01-02"), now.Format("2006-01-02"))
	for provider, subscription := range cfg.Subscriptions {
		equiv := byProvider[provider]
		if equiv <= 0 || subscription <= 0 {
			continue
		}
		summary.Entries = append(summary.Entries, presentation.ROIEntry{
			Provider:        provider,
			EquivalentCost:  equiv,
			SubscriptionUSD: subscription,
			Multiple:        equiv / subscription,
		})
		summary.EquivalentCost += equiv
		summary.SubscriptionCost += subscription
	}
	sort.Slice(summary.Entries, func(i, j int) bool {
		return summary.Entries[i].Provider < summary.Entries[j].Provider
	})
	if summary.SubscriptionCost > 0 {
		summary.Multiple = summary.EquivalentCost / summary.SubscriptionCost
	}
	presentation.PrintROISummary(summary)
}

func reportPeriodStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func localHistoryProviders() []ai.HistoryProvider {
	return []ai.HistoryProvider{
		&aiclaude.HistoryProvider{},
		&aicodex.HistoryProvider{},
	}
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
	aiTokensCmd.Flags().StringVar(&aiTokensProvider, "provider", "", "Only these providers (comma-separated): claude, codex")
	aiTokensCmd.Flags().StringVar(&aiTokensSince, "since", "", "Only records on or after this date (YYYY-MM-DD or RFC3339)")
	aiTokensCmd.Flags().StringVar(&aiTokensUntil, "until", "", "Only records on or before this date (YYYY-MM-DD or RFC3339)")
	aiTokensCmd.Flags().StringVar(&aiTokensGroupBy, "group-by", "provider,model", "Group by: provider, model, provider,model, date, session,model")
	aiTokensCmd.Flags().StringVar(&aiTokensSortBy, "sort-by", "", "Sort by: cluster, cost, tokens, date, group (default cluster desc, grouping rows by provider; date asc for --group-by date)")
	aiTokensCmd.Flags().StringVar(&aiTokensSortOrder, "sort-order", "", "Sort order: asc, desc")
	aiTokensCmd.Flags().BoolVar(&aiTokensShowTotalTokens, "show-total-tokens", false, "Show the aggregate token total column; hidden by default because cache reads can dominate raw totals")
	aiCmd.AddCommand(aiUsageCmd)
	aiCmd.AddCommand(aiTokensCmd)
	rootCmd.AddCommand(aiCmd)
}
