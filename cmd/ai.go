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
	"github.com/jyablonski/arc/internal/skills"
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
	aiSessionsProvider      string
	aiSessionsSince         string
	aiSessionsUntil         string
	aiSessionsLimit         int
	aiSessionsSearch        string
	aiSessionsResume        bool
	aiHealthProvider        string
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

var aiHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check AI tool auth, tooling, and local config offline",
	Long: `Runs offline health checks across the AI toolchain: whether Claude, Codex,
and Cursor are authenticated (and whether their tokens are live or expired),
whether the CLIs are on PATH, whether the shared skills/rules configs are in
sync, and whether the pricing cache is fresh.

Fully offline: unlike "arc ai usage" it makes no network calls and never
refreshes a token. Exits non-zero if any check fails (warnings do not fail).`,
	RunE: runAIHealth,
}

var aiSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List recent local Claude and Codex sessions",
	Long: `Lists recent Claude Code and Codex sessions, newest first, from the same
local JSONL logs that back "arc ai tokens". Shows each session's project,
model, message count, token total, and a title or first-prompt preview.

Fully offline: no network calls and nothing is written. Use --resume to also
print the command that reopens each session in its own tool.`,
	RunE: runAISessions,
}

func runAIUsage(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters, err := parseProviderFilter(aiUsageProvider, ai.ValidateProviderFilters)
	if err != nil {
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

	filters, err := parseProviderFilter(aiTokensProvider, ai.ValidateHistoryProviderFilters)
	if err != nil {
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

func runAIHealth(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters, err := parseProviderFilter(aiHealthProvider, ai.ValidateProviderFilters)
	if err != nil {
		return err
	}

	checks := ai.RunHealthCheckers(cmd.Context(), localHealthCheckers(), filters)
	if aiHealthProvider == "" {
		checks = append(checks, globalHealthChecks()...)
	}
	report := ai.HealthReport{FetchedAt: time.Now(), Checks: checks}

	if jsonOut {
		if err := ai.EncodeHealthJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		presentation.PrintHealth(report)
	}
	if report.HasFailure() {
		return arcerrs.ErrHealthCheckFailed
	}
	return nil
}

func localHealthCheckers() []ai.HealthChecker {
	return []ai.HealthChecker{
		&aiclaude.Provider{},
		&aicodex.Provider{},
		&aicursor.Provider{},
	}
}

// globalHealthChecks are the machine-wide checks not tied to a single provider:
// pricing-cache freshness and shared skills/rules sync state.
func globalHealthChecks() []ai.HealthCheck {
	checks := []ai.HealthCheck{pricingCacheCheck()}
	return append(checks, configSyncChecks()...)
}

func pricingCacheCheck() ai.HealthCheck {
	c := ai.HealthCheck{Category: "pricing", Name: "pricing"}
	cf, ok, err := ai.ReadPricingCacheFile()
	switch {
	case err != nil:
		c.Status = ai.HealthWarn
		c.Detail = "pricing cache unreadable: " + err.Error()
		c.Hint = "run 'arc ai pricing'"
	case !ok:
		c.Status = ai.HealthWarn
		c.Detail = "no pricing cache; 'arc ai tokens' falls back to built-in defaults"
		c.Hint = "run 'arc ai pricing' to refresh"
	default:
		age := time.Since(cf.FetchedAt)
		if age > 30*24*time.Hour {
			c.Status = ai.HealthWarn
			c.Detail = fmt.Sprintf("pricing cache is %d days old", int(age.Hours()/24))
			c.Hint = "run 'arc ai pricing'"
		} else {
			c.Status = ai.HealthOK
			c.Detail = fmt.Sprintf("%d models, refreshed %s", len(cf.Prices), cf.FetchedAt.Format("2006-01-02"))
		}
	}
	return c
}

func configSyncChecks() []ai.HealthCheck {
	paths := skills.DefaultPaths()
	m := skills.New(skills.Config{})
	var checks []ai.HealthCheck

	if _, err := os.Stat(paths.SkillsRoot); err == nil {
		if res, err := m.List(); err != nil {
			checks = append(checks, ai.HealthCheck{Category: "config", Name: "skills", Status: ai.HealthWarn, Detail: "cannot list skills: " + err.Error()})
		} else {
			bad := 0
			for _, s := range res.Skills {
				for _, st := range s.Providers {
					if st == skills.StatusDangling || st == skills.StatusConflict {
						bad++
					}
				}
			}
			if bad > 0 {
				checks = append(checks, ai.HealthCheck{Category: "config", Name: "skills", Status: ai.HealthWarn, Detail: fmt.Sprintf("%d skill link(s) dangling or conflicted", bad), Hint: "run 'arc skills sync' (or 'arc skills prune')"})
			} else {
				checks = append(checks, ai.HealthCheck{Category: "config", Name: "skills", Status: ai.HealthOK, Detail: fmt.Sprintf("%d canonical skills linked cleanly", len(res.Skills))})
			}
		}
	}

	if _, err := os.Stat(paths.RulesFile); err == nil {
		drift := 0
		for _, e := range m.StatusRules().Providers {
			if e.Status != skills.StatusOK {
				drift++
			}
		}
		if drift > 0 {
			checks = append(checks, ai.HealthCheck{Category: "config", Name: "rules", Status: ai.HealthWarn, Detail: fmt.Sprintf("%d provider(s) out of sync with AGENTS.md", drift), Hint: "run 'arc rules sync'"})
		} else {
			checks = append(checks, ai.HealthCheck{Category: "config", Name: "rules", Status: ai.HealthOK, Detail: "AGENTS.md linked in all providers"})
		}
	}

	return checks
}

func runAISessions(cmd *cobra.Command, args []string) error {
	_ = args
	jsonOut, _ := cmd.Flags().GetBool("json")

	filters, err := parseProviderFilter(aiSessionsProvider, ai.ValidateHistoryProviderFilters)
	if err != nil {
		return err
	}
	if aiSessionsLimit < 0 {
		return fmt.Errorf("--limit must be >= 0")
	}

	histOpts, err := parseHistoryOptions(aiSessionsSince, aiSessionsUntil)
	if err != nil {
		return err
	}
	opts := ai.SessionOptions{
		Since:  histOpts.Since,
		Until:  histOpts.Until,
		Limit:  aiSessionsLimit,
		Search: strings.TrimSpace(aiSessionsSearch),
	}

	report := ai.RunSessionProviders(cmd.Context(), localSessionProviders(), filters, opts)
	report.FetchedAt = time.Now()

	if jsonOut {
		if err := ai.EncodeSessionsJSON(os.Stdout, report); err != nil {
			return err
		}
		return ai.ExitErrorIfAllSessionProvidersFailed(report)
	}
	presentation.PrintSessions(report, presentation.SessionsPrintOptions{ShowResume: aiSessionsResume})
	return ai.ExitErrorIfAllSessionProvidersFailed(report)
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

// parseProviderFilter parses a --provider CSV, rejecting a non-empty value that
// resolves to nothing, and validates the names against the given validator.
func parseProviderFilter(raw string, validate func([]string) error) ([]string, error) {
	filters := ai.ParseProviderCSV(raw)
	if raw != "" && len(filters) == 0 {
		return nil, arcerrs.ErrEmptyProviderFilter
	}
	if err := validate(filters); err != nil {
		return nil, err
	}
	return filters, nil
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

func localSessionProviders() []ai.SessionProvider {
	return []ai.SessionProvider{
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
	aiSessionsCmd.Flags().StringVar(&aiSessionsProvider, "provider", "", "Only these providers (comma-separated): claude, codex")
	aiSessionsCmd.Flags().StringVar(&aiSessionsSince, "since", "", "Only sessions active on or after this date (YYYY-MM-DD or RFC3339)")
	aiSessionsCmd.Flags().StringVar(&aiSessionsUntil, "until", "", "Only sessions started on or before this date (YYYY-MM-DD or RFC3339)")
	aiSessionsCmd.Flags().IntVar(&aiSessionsLimit, "limit", 20, "Show at most this many sessions (0 for no limit)")
	aiSessionsCmd.Flags().StringVar(&aiSessionsSearch, "search", "", "Only sessions whose project, title, or id contains this text")
	aiSessionsCmd.Flags().BoolVar(&aiSessionsResume, "resume", false, "Also print the command to resume each session")
	aiHealthCmd.Flags().StringVar(&aiHealthProvider, "provider", "", "Only these providers (comma-separated): claude, codex, cursor")
	aiCmd.AddCommand(aiUsageCmd)
	aiCmd.AddCommand(aiTokensCmd)
	aiCmd.AddCommand(aiPricingCmd)
	aiCmd.AddCommand(aiSessionsCmd)
	aiCmd.AddCommand(aiHealthCmd)
	rootCmd.AddCommand(aiCmd)
}
