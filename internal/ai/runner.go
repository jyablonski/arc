package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"
)

func RunProviders(ctx context.Context, providers []Provider, filterNames []string) AggregateReport {
	filter := normalizeFilter(filterNames)
	var selected []Provider
	for _, p := range providers {
		if len(filter) == 0 {
			selected = append(selected, p)
			continue
		}
		n := strings.ToLower(strings.TrimSpace(p.Name()))
		if filter[n] {
			selected = append(selected, p)
		}
	}
	if len(selected) == 0 {
		return AggregateReport{Providers: nil}
	}

	out := make([]ProviderResult, len(selected))
	g, ctx := errgroup.WithContext(ctx)
	for i, p := range selected {
		i, p := i, p
		g.Go(func() error {
			rep, err := p.Usage(ctx)
			if rep.Windows == nil {
				rep.Windows = []UsageWindow{}
			}
			pr := ProviderResult{Name: p.Name(), Report: rep}
			if err != nil {
				pr.OK = false
				pr.Error = err.Error()
				pr.Hint = hintFor(p.Name(), err)
			} else {
				pr.OK = true
			}
			out[i] = pr
			return nil
		})
	}
	_ = g.Wait()
	return AggregateReport{Providers: out}
}

func normalizeFilter(names []string) map[string]bool {
	m := make(map[string]bool)
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" {
			m[n] = true
		}
	}
	return m
}

func hintFor(provider string, err error) string {
	switch strings.ToLower(provider) {
	case "claude":
		return "check ~/.claude/.credentials.json (accessToken + refreshToken), macOS Keychain if applicable; OAuth refresh hits console.anthropic.com/v1/oauth/token — rate limits possible; header anthropic-beta oauth-2025-04-20 if usage API shape changed"
	case "codex":
		return "install Codex CLI (npm i -g @openai/codex), run codex login, and ensure codex is on PATH; see OpenAI Codex App Server docs for account/rateLimits/read"
	case "cursor":
		return "ensure Cursor is logged in; token lives in state.vscdb (cursorAuth/accessToken); dashboard API may change — see cursor-usage-monitor upstream"
	default:
		_ = err
		return ""
	}
}

func CombineErrors(agg AggregateReport) error {
	var parts []string
	for _, p := range agg.Providers {
		if !p.OK && p.Error != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Error))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
