package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func ParseProviderCSV(s string) []string {
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

func ValidateProviderFilters(filters []string) error {
	known := map[string]struct{}{"claude": {}, "codex": {}, "cursor": {}}
	for _, f := range filters {
		if _, ok := known[f]; !ok {
			return fmt.Errorf("unknown provider %q (claude, codex, cursor)", f)
		}
	}
	return nil
}

func ValidateHistoryProviderFilters(filters []string) error {
	known := map[string]struct{}{"claude": {}, "codex": {}}
	for _, f := range filters {
		if _, ok := known[f]; !ok {
			return fmt.Errorf("unknown provider %q (claude, codex)", f)
		}
	}
	return nil
}

func EncodeAggregateJSON(w io.Writer, agg AggregateReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(agg)
}

func ExitErrorIfAllProvidersFailed(agg AggregateReport) error {
	if len(agg.Providers) == 0 {
		return ErrNoProvidersSelected
	}
	anyOK := false
	for _, p := range agg.Providers {
		if p.OK {
			anyOK = true
			break
		}
	}
	if !anyOK {
		return CombineErrors(agg)
	}
	return nil
}
