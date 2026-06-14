package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/ai"
	aiclaude "github.com/jyablonski/arc/internal/ai/claude"
	aicodex "github.com/jyablonski/arc/internal/ai/codex"
	aicursor "github.com/jyablonski/arc/internal/ai/cursor"
	"github.com/jyablonski/arc/internal/ai/presentation"
	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
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
	aiPricingSource         string
	aiPricingDryRun         bool
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

var aiPricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Refresh the local model pricing cache used by 'arc ai tokens'",
	Long: `Downloads a current model pricing table and writes it to the local cache at
~/.cache/arc/ai-pricing.json. This is the only networked command under "arc ai"
besides "arc ai usage": "arc ai tokens" itself stays offline and reads whatever
this command last cached.

Pricing is layered, highest priority first:
  1. ~/.config/arc/ai-pricing.json   hand-edited overrides (you maintain)
  2. ~/.cache/arc/ai-pricing.json     fetched by this command
  3. built-in defaults                shipped with arc

The default source is LiteLLM's community pricing table; pass --source to use
your own JSON in the same format. A brand-new model the source does not list yet
can be priced immediately by adding it to the override file, which always wins.`,
	RunE: runAIPricing,
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
	report := ai.RunHistoryProviders(cmd.Context(), localHistoryProviders(), filters, opts, ai.NewLayeredPricer(), aiTokensGroupBy)
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

func runAIPricing(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	source := aiPricingSource
	if source == "" {
		source = ai.DefaultPricingSource
	}

	client := &http.Client{Timeout: 30 * time.Second}
	prices, err := ai.FetchPricing(cmd.Context(), source, client)
	if err != nil {
		return err
	}

	prev, _ := ai.ReadPricingCache()
	added := newModelKeys(prev, prices)
	now := time.Now()

	if !aiPricingDryRun {
		if err := ai.WritePricingCache(now, source, prices); err != nil {
			return err
		}
	}

	if jsonOut {
		path, _ := ai.PricingCachePath()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"source":      source,
			"fetched_at":  now,
			"cache_path":  path,
			"dry_run":     aiPricingDryRun,
			"model_count": len(prices),
			"new_models":  added,
			"prices":      prices,
		})
	}

	return printPricingSummary(source, prices, added, now)
}

func printPricingSummary(source string, prices map[string]ai.ModelPrice, added []string, now time.Time) error {
	path, _ := ai.PricingCachePath()
	overridePath, _ := ai.PricingOverridePath()

	if aiPricingDryRun {
		output.Info(fmt.Sprintf("Dry run: %d models from %s (cache not written)", len(prices), source))
	} else {
		output.Success(fmt.Sprintf("Cached %d models from %s", len(prices), source))
		output.Print(fmt.Sprintf("  written to %s at %s", path, now.Format(time.RFC3339)))
	}
	if len(added) > 0 {
		shown := added
		if len(shown) > 10 {
			shown = shown[:10]
		}
		output.Print(fmt.Sprintf("  new since last refresh (%d): %s", len(added), strings.Join(shown, ", ")))
		if len(added) > len(shown) {
			output.Print(fmt.Sprintf("  …and %d more", len(added)-len(shown)))
		}
	}
	output.Print(fmt.Sprintf("  hand-edit %s to add or override models (always wins)", overridePath))
	return nil
}

// newModelKeys returns the price keys present in next but not in prev, sorted.
func newModelKeys(prev, next map[string]ai.ModelPrice) []string {
	var added []string
	for k := range next {
		if _, ok := prev[k]; !ok {
			added = append(added, k)
		}
	}
	sort.Strings(added)
	return added
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
	report := ai.RunHistoryProviders(ctx, localHistoryProviders(), filters, ai.CurrentMonthOptions(now), ai.NewLayeredPricer(), "provider")
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
	aiPricingCmd.Flags().StringVar(&aiPricingSource, "source", "", "Pricing table URL in LiteLLM JSON format (default LiteLLM's table)")
	aiPricingCmd.Flags().BoolVar(&aiPricingDryRun, "dry-run", false, "Fetch and report without writing the cache")
	aiCmd.AddCommand(aiUsageCmd)
	aiCmd.AddCommand(aiTokensCmd)
	aiCmd.AddCommand(aiPricingCmd)
	rootCmd.AddCommand(aiCmd)
}
