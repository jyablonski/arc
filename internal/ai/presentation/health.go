package presentation

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

// healthSectionRank orders health rows by provider, matching the provider order
// used by "arc ai usage"/"ai tokens"; machine-wide checks ("local") come last.
var healthSectionRank = map[string]int{"claude": 0, "codex": 1, "cursor": 2, "local": 3}

func PrintHealth(report ai.HealthReport) {
	oldOut := color.Output
	color.Output = os.Stdout
	defer func() { color.Output = oldOut }()

	output.SectionAccent("AI health", providerAccent("claude"))
	if len(report.Checks) == 0 {
		output.Info("no health checks ran")
		return
	}

	checks := append([]ai.HealthCheck(nil), report.Checks...)
	sort.SliceStable(checks, func(i, j int) bool {
		return healthRank(checks[i]) < healthRank(checks[j])
	})

	headers := []string{"status", "provider", "check", "detail"}
	rows := make([][]string, 0, len(checks))
	for _, c := range checks {
		section, label := healthSection(c)
		provider := strings.ToLower(section)
		rows = append(rows, []string{
			healthMark(c.Status),
			providerAccent(provider).Sprint(provider),
			label,
			c.Detail,
		})
	}
	output.Table(headers, rows)

	printHealthHints(checks)
}

// printHealthHints lists the actionable hint for each non-OK check beneath the
// table. The common all-green case prints nothing.
func printHealthHints(checks []ai.HealthCheck) {
	printedHeader := false
	for _, c := range checks {
		if c.Status == ai.HealthOK || c.Hint == "" {
			continue
		}
		if !printedHeader {
			output.Print("")
			printedHeader = true
		}
		section, label := healthSection(c)
		output.Print(fmt.Sprintf("  %s %s/%s: %s", healthMark(c.Status), strings.ToLower(section), label, c.Hint))
	}
}

// healthSection maps a check to its display section and row label: provider
// auth/tooling checks group under the provider; everything else is machine-wide.
func healthSection(c ai.HealthCheck) (section, label string) {
	switch c.Category {
	case "auth", "tooling":
		return c.Name, c.Category
	default:
		return "local", c.Name
	}
}

func healthRank(c ai.HealthCheck) int {
	section, _ := healthSection(c)
	if r, ok := healthSectionRank[strings.ToLower(section)]; ok {
		return r
	}
	return len(healthSectionRank)
}

func healthMark(s ai.HealthStatus) string {
	switch s {
	case ai.HealthOK:
		return color.New(color.FgGreen).Sprint("✓")
	case ai.HealthWarn:
		return color.New(color.FgYellow).Sprint("!")
	case ai.HealthFail:
		return color.New(color.FgRed).Sprint("✗")
	default:
		return "?"
	}
}
