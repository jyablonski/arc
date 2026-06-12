package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// ErrNoProvidersSelected is returned by the exit-status helpers when a report
// contains no providers (e.g. an over-narrow --provider filter).
var ErrNoProvidersSelected = errors.New("no providers selected")

type TokenBreakdown struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	Reasoning  int64 `json:"reasoning"`
}

func (t TokenBreakdown) Total() int64 {
	return t.Input + t.Output + t.CacheRead + t.CacheWrite + t.Reasoning
}

func (t TokenBreakdown) Add(other TokenBreakdown) TokenBreakdown {
	return TokenBreakdown{
		Input:      t.Input + other.Input,
		Output:     t.Output + other.Output,
		CacheRead:  t.CacheRead + other.CacheRead,
		CacheWrite: t.CacheWrite + other.CacheWrite,
		Reasoning:  t.Reasoning + other.Reasoning,
	}
}

type TokenRecord struct {
	Provider      string         `json:"provider"`
	Model         string         `json:"model"`
	SessionID     string         `json:"session_id"`
	Timestamp     time.Time      `json:"timestamp"`
	Tokens        TokenBreakdown `json:"tokens"`
	CostUSD       float64        `json:"cost_usd"`
	PricingSource string         `json:"pricing_source,omitempty"`
}

type HistoryOptions struct {
	Since *time.Time
	Until *time.Time
}

func (o HistoryOptions) Contains(t time.Time) bool {
	if o.Since != nil && t.Before(*o.Since) {
		return false
	}
	if o.Until != nil && t.After(*o.Until) {
		return false
	}
	return true
}

func ResolveHomeDir(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return override, nil
	}
	return os.UserHomeDir()
}

//go:generate go tool moq -rm -out historyprovider_moq.go . HistoryProvider

type HistoryProvider interface {
	Name() string
	LocalUsage(ctx context.Context, opts HistoryOptions) ([]TokenRecord, error)
}

type HistoryProviderResult struct {
	Name    string        `json:"name"`
	OK      bool          `json:"ok"`
	Error   string        `json:"error,omitempty"`
	Hint    string        `json:"hint,omitempty"`
	Records []TokenRecord `json:"records"`
}

type UsageGroup struct {
	Provider      string         `json:"provider,omitempty"`
	Model         string         `json:"model,omitempty"`
	Date          string         `json:"date,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	Tokens        TokenBreakdown `json:"tokens"`
	CostUSD       float64        `json:"cost_usd"`
	PricingSource string         `json:"pricing_source,omitempty"`
	Records       int            `json:"records"`
}

type HistoryReport struct {
	FetchedAt time.Time               `json:"fetched_at"`
	GroupBy   string                  `json:"group_by"`
	SortBy    string                  `json:"sort_by,omitempty"`
	SortOrder string                  `json:"sort_order,omitempty"`
	Providers []HistoryProviderResult `json:"providers"`
	Groups    []UsageGroup            `json:"groups"`
	Total     UsageGroup              `json:"total"`
}

func RunHistoryProviders(ctx context.Context, providers []HistoryProvider, filters []string, opts HistoryOptions, pricer Pricer, groupBy string) HistoryReport {
	filter := normalizeFilter(filters)
	selected := make([]HistoryProvider, 0, len(providers))
	for _, p := range providers {
		if len(filter) == 0 || filter[strings.ToLower(strings.TrimSpace(p.Name()))] {
			selected = append(selected, p)
		}
	}

	results := make([]HistoryProviderResult, 0, len(selected))
	var records []TokenRecord
	for _, p := range selected {
		rec, err := p.LocalUsage(ctx, opts)
		if rec == nil {
			rec = []TokenRecord{}
		}
		res := HistoryProviderResult{Name: p.Name(), Records: rec}
		if err != nil {
			res.Error = err.Error()
			res.Hint = historyHintFor(p.Name())
		} else {
			res.OK = true
		}
		for i := range rec {
			rec[i].Provider = strings.ToLower(strings.TrimSpace(rec[i].Provider))
			rec[i].Model = strings.TrimSpace(rec[i].Model)
			if rec[i].PricingSource == "" {
				rec[i].CostUSD, rec[i].PricingSource = pricer.Cost(rec[i].Model, rec[i].Tokens)
			}
			records = append(records, rec[i])
		}
		res.Records = rec
		results = append(results, res)
	}

	groups := GroupTokenRecords(records, groupBy)
	total := totalGroup(groups)
	return HistoryReport{
		FetchedAt: time.Now(),
		GroupBy:   normalizeHistoryGroupBy(groupBy),
		Providers: results,
		Groups:    groups,
		Total:     total,
	}
}

func historyHintFor(provider string) string {
	switch strings.ToLower(provider) {
	case "claude":
		return "Claude Code local usage is read from ~/.claude/projects and ~/.claude/transcripts JSONL files"
	case "codex":
		return "Codex local usage is read from ~/.codex/sessions JSONL token_count events"
	case "cursor":
		return "Cursor does not expose local token transcripts in this implementation; use its dashboard usage API for quota/spend"
	default:
		return ""
	}
}

func normalizeHistoryGroupBy(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "provider,model":
		return "provider,model"
	case "provider":
		return "provider"
	case "model":
		return "model"
	case "day", "date":
		return "date"
	case "session", "session,model":
		return "session,model"
	default:
		return "provider,model"
	}
}

func ValidateHistoryGroupBy(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "provider", "model", "provider,model", "day", "date", "session", "session,model":
		return nil
	default:
		return fmt.Errorf("unknown group-by %q (provider, model, provider,model, date, session,model)", s)
	}
}

func ValidateHistorySort(sortBy, sortOrder string) error {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "", "cost", "tokens", "date", "group", "cluster":
	default:
		return fmt.Errorf("unknown sort-by %q (cluster, cost, tokens, date, group)", sortBy)
	}
	switch strings.ToLower(strings.TrimSpace(sortOrder)) {
	case "", "asc", "desc":
	default:
		return fmt.Errorf("unknown sort-order %q (asc, desc)", sortOrder)
	}
	return nil
}

func NormalizeHistorySort(groupBy, sortBy, sortOrder string) (string, string) {
	groupBy = normalizeHistoryGroupBy(groupBy)
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	sortOrder = strings.ToLower(strings.TrimSpace(sortOrder))
	if sortBy == "" {
		// Default clusters rows into per-provider bands (providers ordered by
		// combined cost, models by cost within), which reads cleanly alongside
		// the per-provider row coloring. Date tables stay chronological.
		sortBy = "cluster"
		if groupBy == "date" {
			sortBy = "date"
		}
	}
	if sortOrder == "" {
		sortOrder = "desc"
		if groupBy == "date" && sortBy == "date" {
			sortOrder = "asc"
		}
	}
	return sortBy, sortOrder
}

func GroupTokenRecords(records []TokenRecord, groupBy string) []UsageGroup {
	groupBy = normalizeHistoryGroupBy(groupBy)
	byKey := map[string]UsageGroup{}
	for _, r := range records {
		key, g := groupKey(r, groupBy)
		existing := byKey[key]
		if existing.Provider == "" && existing.Model == "" && existing.Date == "" && existing.SessionID == "" {
			existing = g
		}
		existing.Tokens = existing.Tokens.Add(r.Tokens)
		existing.CostUSD += r.CostUSD
		existing.Records++
		existing.PricingSource = mergePricingSource(existing.PricingSource, r.PricingSource)
		byKey[key] = existing
	}
	out := make([]UsageGroup, 0, len(byKey))
	for _, g := range byKey {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		if out[i].SessionID != out[j].SessionID {
			return out[i].SessionID < out[j].SessionID
		}
		return out[i].Model < out[j].Model
	})
	return out
}

func SortUsageGroups(groups []UsageGroup, groupBy, sortBy, sortOrder string) {
	groupBy = normalizeHistoryGroupBy(groupBy)
	sortBy, sortOrder = NormalizeHistorySort(groupBy, sortBy, sortOrder)
	asc := sortOrder == "asc"
	if sortBy == "cluster" {
		sortClusteredByProvider(groups, asc)
		return
	}
	sort.SliceStable(groups, func(i, j int) bool {
		cmp := compareUsageGroup(groups[i], groups[j], sortBy, groupBy)
		if asc {
			return cmp < 0
		}
		return cmp > 0
	})
}

// sortClusteredByProvider groups rows into contiguous per-provider bands ordered
// by each provider's combined cost, with rows sorted by cost within a band.
// Groups without a provider (e.g. --group-by model) collapse to a plain cost
// sort. The order honors sortOrder; provider name breaks ties deterministically.
func sortClusteredByProvider(groups []UsageGroup, asc bool) {
	providerCost := map[string]float64{}
	for _, g := range groups {
		providerCost[g.Provider] += g.CostUSD
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.Provider != b.Provider {
			if ca, cb := providerCost[a.Provider], providerCost[b.Provider]; ca != cb {
				if asc {
					return ca < cb
				}
				return ca > cb
			}
			return a.Provider < b.Provider
		}
		if a.CostUSD != b.CostUSD {
			if asc {
				return a.CostUSD < b.CostUSD
			}
			return a.CostUSD > b.CostUSD
		}
		return false
	})
}

func compareUsageGroup(a, b UsageGroup, sortBy, groupBy string) int {
	switch sortBy {
	case "tokens":
		return cmpInt64(a.Tokens.Total(), b.Tokens.Total())
	case "date":
		return strings.Compare(groupDate(a), groupDate(b))
	case "group":
		return strings.Compare(groupNameForSort(a, groupBy), groupNameForSort(b, groupBy))
	default:
		return cmpFloat(a.CostUSD, b.CostUSD)
	}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func groupDate(g UsageGroup) string {
	if g.Date != "" {
		return g.Date
	}
	return ""
}

func groupNameForSort(g UsageGroup, groupBy string) string {
	switch normalizeHistoryGroupBy(groupBy) {
	case "provider":
		return g.Provider
	case "model":
		return g.Model
	case "date":
		return g.Date
	case "session,model":
		return g.Provider + "/" + g.SessionID + "/" + g.Model
	default:
		return g.Provider + "/" + g.Model
	}
}

func groupKey(r TokenRecord, groupBy string) (string, UsageGroup) {
	switch groupBy {
	case "provider":
		return r.Provider, UsageGroup{Provider: r.Provider}
	case "model":
		return r.Model, UsageGroup{Model: r.Model}
	case "date":
		date := r.Timestamp.Local().Format("2006-01-02")
		return date, UsageGroup{Date: date}
	case "session,model":
		key := r.Provider + "\x00" + r.SessionID + "\x00" + r.Model
		return key, UsageGroup{Provider: r.Provider, SessionID: r.SessionID, Model: r.Model}
	default:
		key := r.Provider + "\x00" + r.Model
		return key, UsageGroup{Provider: r.Provider, Model: r.Model}
	}
}

func mergePricingSource(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" || a == b {
		return a
	}
	if strings.Contains(a, "mixed") {
		return a
	}
	return "mixed"
}

func totalGroup(groups []UsageGroup) UsageGroup {
	var total UsageGroup
	for _, g := range groups {
		total.Tokens = total.Tokens.Add(g.Tokens)
		total.CostUSD += g.CostUSD
		total.Records += g.Records
		total.PricingSource = mergePricingSource(total.PricingSource, g.PricingSource)
	}
	return total
}

func CurrentMonthOptions(now time.Time) HistoryOptions {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return HistoryOptions{Since: &start, Until: &now}
}

func EncodeHistoryJSON(w io.Writer, report HistoryReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func ExitErrorIfAllHistoryProvidersFailed(report HistoryReport) error {
	if len(report.Providers) == 0 {
		return ErrNoProvidersSelected
	}
	anyOK := false
	for _, p := range report.Providers {
		if p.OK {
			anyOK = true
			break
		}
	}
	if anyOK {
		return nil
	}
	var parts []string
	for _, p := range report.Providers {
		if p.Error != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Error))
		}
	}
	return errors.New(strings.Join(parts, "; "))
}
