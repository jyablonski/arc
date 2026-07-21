package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// SessionSummary is one Claude or Codex session folded down to the fields that
// make it findable and resumable. It is built offline from the same local
// JSONL logs that back "arc ai tokens".
type SessionSummary struct {
	Provider  string         `json:"provider"`
	SessionID string         `json:"session_id"`
	ResumeID  string         `json:"resume_id,omitempty"`
	Project   string         `json:"project,omitempty"`
	Title     string         `json:"title,omitempty"`
	Model     string         `json:"model,omitempty"`
	Messages  int            `json:"messages"`
	Tokens    TokenBreakdown `json:"tokens"`
	StartedAt time.Time      `json:"started_at"`
	LastAt    time.Time      `json:"last_at"`
}

//go:generate go tool moq -rm -out sessionprovider_moq.go . SessionProvider

// SessionProvider lists the local sessions for one AI tool. Implementations
// read on-disk logs only and never touch the network.
type SessionProvider interface {
	Name() string
	Sessions(ctx context.Context) ([]SessionSummary, error)
}

type SessionProviderResult struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

// SessionOptions filters and bounds the session list. Since/Until are matched
// against each session's activity window; Limit caps the newest-first result.
type SessionOptions struct {
	Since  *time.Time
	Until  *time.Time
	Limit  int
	Search string
}

type SessionReport struct {
	FetchedAt time.Time               `json:"fetched_at"`
	Providers []SessionProviderResult `json:"providers"`
	Sessions  []SessionSummary        `json:"sessions"`
}

// RunSessionProviders gathers sessions from each selected provider, isolating
// per-provider failures, then filters, sorts newest-first, and applies the limit.
func RunSessionProviders(ctx context.Context, providers []SessionProvider, filters []string, opts SessionOptions) SessionReport {
	filter := normalizeFilter(filters)
	selected := make([]SessionProvider, 0, len(providers))
	for _, p := range providers {
		if len(filter) == 0 || filter[strings.ToLower(strings.TrimSpace(p.Name()))] {
			selected = append(selected, p)
		}
	}

	report := SessionReport{Providers: make([]SessionProviderResult, 0, len(selected))}
	var all []SessionSummary
	for _, p := range selected {
		sess, err := p.Sessions(ctx)
		res := SessionProviderResult{Name: p.Name()}
		if err != nil {
			res.Error = err.Error()
			res.Hint = historyHintFor(p.Name())
		} else {
			res.OK = true
		}
		report.Providers = append(report.Providers, res)
		all = append(all, sess...)
	}

	filtered := make([]SessionSummary, 0, len(all))
	for _, s := range all {
		if opts.Since != nil && s.LastAt.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && s.StartedAt.After(*opts.Until) {
			continue
		}
		if opts.Search != "" && !sessionMatches(s, opts.Search) {
			continue
		}
		filtered = append(filtered, s)
	}

	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if !a.LastAt.Equal(b.LastAt) {
			return a.LastAt.After(b.LastAt)
		}
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.After(b.StartedAt)
		}
		return a.SessionID < b.SessionID
	})

	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	report.Sessions = filtered
	return report
}

// FoldSessions collapses token records into per-session accumulators keyed by
// SessionID. Providers reuse it so the (audited) token math from LocalUsage
// drives token totals, message counts, and the activity window; provider-specific
// metadata (project, title, resume id) is layered on afterward. Model is taken
// from the most recent record in the session.
func FoldSessions(records []TokenRecord) map[string]*SessionSummary {
	m := make(map[string]*SessionSummary, len(records))
	for _, r := range records {
		s := m[r.SessionID]
		if s == nil {
			s = &SessionSummary{Provider: r.Provider, SessionID: r.SessionID, StartedAt: r.Timestamp, LastAt: r.Timestamp}
			m[r.SessionID] = s
		}
		s.Tokens = s.Tokens.Add(r.Tokens)
		s.Messages++
		if model := strings.TrimSpace(r.Model); model != "" && (s.Model == "" || !r.Timestamp.Before(s.LastAt)) {
			s.Model = model
		}
		if r.Timestamp.IsZero() {
			continue
		}
		if s.StartedAt.IsZero() || r.Timestamp.Before(s.StartedAt) {
			s.StartedAt = r.Timestamp
		}
		if r.Timestamp.After(s.LastAt) {
			s.LastAt = r.Timestamp
		}
	}
	return m
}

// FirstLine returns the first non-blank line of s, trimmed. Providers use it to
// turn a multi-line first prompt into a one-line session title.
func FirstLine(s string) string {
	s = strings.TrimSpace(s)
	if before, _, found := strings.Cut(s, "\n"); found {
		return strings.TrimSpace(before)
	}
	return s
}

func sessionMatches(s SessionSummary, query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(s.Project), q) ||
		strings.Contains(strings.ToLower(s.Title), q) ||
		strings.Contains(strings.ToLower(s.SessionID), q)
}

func EncodeSessionsJSON(w io.Writer, report SessionReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// ExitErrorIfAllSessionProvidersFailed returns a combined error only when every
// selected provider errored, mirroring the token-history exit semantics.
func ExitErrorIfAllSessionProvidersFailed(report SessionReport) error {
	if len(report.Providers) == 0 {
		return ErrNoProvidersSelected
	}
	for _, p := range report.Providers {
		if p.OK {
			return nil
		}
	}
	var parts []string
	for _, p := range report.Providers {
		if p.Error != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Error))
		}
	}
	return errors.New(strings.Join(parts, "; "))
}
