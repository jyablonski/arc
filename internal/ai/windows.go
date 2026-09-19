package ai

import "strings"

// NormalizeWindows fills UsageWindow.Window with one vocabulary across
// providers ("5 hour", "7 day", "month · api", …) while Label keeps the
// provider's own wording for --json consumers.
func NormalizeWindows(agg *AggregateReport) {
	for i := range agg.Providers {
		p := &agg.Providers[i]
		for j := range p.Report.Windows {
			w := &p.Report.Windows[j]
			w.Window = NormalizeWindow(p.Name, w.Label)
		}
	}
}

// NormalizeWindow maps a provider window label onto arc's vocabulary.
func NormalizeWindow(provider, label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	switch provider {
	case "claude":
		// "7 day (all models)" is the plain weekly window; model-scoped
		// weekly windows keep their scope as a suffix.
		if rest, ok := strings.CutPrefix(lower, "7 day ("); ok {
			scope := strings.TrimSuffix(rest, ")")
			if scope == "all models" {
				return "7 day"
			}
			return "7 day · " + scope
		}
	case "codex":
		if lower == "weekly" {
			return "7 day"
		}
	case "cursor":
		// Cursor meters every pool against the monthly billing cycle.
		scope := map[string]string{
			"total":                  "total",
			"total usage (included)": "included",
			"auto + composer":        "auto",
			"api":                    "api",
			"bonus spend":            "bonus",
			"on-demand budget":       "on-demand",
		}[lower]
		if scope == "" {
			scope = lower
		}
		return "month · " + scope
	}
	return lower
}
