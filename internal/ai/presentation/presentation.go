package presentation

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jyablonski/arc/internal/ai"
	"github.com/jyablonski/arc/internal/output"
)

const (
	colWindowWidth  = 28
	usageBarWidth   = 22
	colRemainWidth  = 8
	colRemainHeader = "% left"
	dotFilled       = "●"
	dotEmpty        = "·"
)

var faintColor = color.New(color.Faint)

func PrintAggregate(agg ai.AggregateReport) {
	oldOut := color.Output
	color.Output = os.Stdout
	defer func() { color.Output = oldOut }()

	now := time.Now()
	pctGreen := color.New(color.FgGreen)
	for _, pr := range agg.Providers {
		output.SectionAccent(capitalize(pr.Name), providerAccent(pr.Name))
		if !pr.OK {
			output.Error(pr.Error)
			if pr.Hint != "" {
				output.Warning(pr.Hint)
			}
			continue
		}
		if len(pr.Report.Windows) == 0 && len(pr.Report.Extra) == 0 {
			output.Info("no usage windows returned")
			continue
		}
		if len(pr.Report.Windows) > 0 {
			fmt.Printf("%s  %s  %s  %s\n",
				faintColor.Sprint(padRunes("window", colWindowWidth)),
				faintColor.Sprint(padRunes("usage", usageBarWidth)),
				faintColor.Sprint(alignPercentCell(colRemainHeader)),
				faintColor.Sprint("resets"),
			)
			accent := providerAccent(pr.Name)
			for _, w := range pr.Report.Windows {
				remainPct := pctRemainingForDisplay(w.PercentUsed)
				fmt.Print(faintColor.Sprint(padRunes(w.Label, colWindowWidth)))
				fmt.Print("  ")
				fmt.Print(renderRemainBar(accent, remainPct, usageBarWidth))
				fmt.Print("  ")
				fmt.Print(formatRemainPercentHuman(w.PercentUsed, pctGreen))
				fmt.Print("  ")
				fmt.Println(faintColor.Sprint(formatResetHuman(w.ResetsAt, now)))
			}
		}
	}
}

func providerAccent(providerName string) *color.Color {
	switch providerName {
	case "claude":
		return color.RGB(255, 133, 31)
	case "codex":
		return color.New(color.FgGreen)
	case "cursor":
		return color.New(color.FgHiMagenta)
	default:
		return color.New(color.FgCyan, color.Bold)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func padRunes(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		if width <= 1 {
			return ""
		}
		return string(r[:width-1]) + "…"
	}
	return s + strings.Repeat(" ", width-len(r))
}

func pctRemainingForDisplay(usedPct float64) float64 {
	if usedPct < 0 {
		return -1
	}
	remain := 100 - usedPct
	if remain < 0 {
		return 0
	}
	if remain > 100 {
		return 100
	}
	return remain
}

func alignPercentCell(s string) string {
	return fmt.Sprintf("%*s", colRemainWidth, s)
}

func renderRemainBar(accent *color.Color, remainPct float64, width int) string {
	switch {
	case remainPct < 0:
		return faintColor.Sprint(strings.Repeat(dotEmpty, width))
	default:
		n := int(math.Round(remainPct / 100 * float64(width)))
		if n > width {
			n = width
		}
		if n < 0 {
			n = 0
		}
		filled := strings.Repeat(dotFilled, n)
		rest := strings.Repeat(dotEmpty, width-n)
		return accent.Sprint(filled) + faintColor.Sprint(rest)
	}
}

func formatRemainPercentHuman(usedPct float64, green *color.Color) string {
	if usedPct < 0 {
		return faintColor.Sprint(alignPercentCell("—"))
	}
	remain := 100 - usedPct
	if remain < 0 {
		remain = 0
	}
	if remain > 100 {
		remain = 100
	}
	return green.Sprint(alignPercentCell(fmt.Sprintf("%.1f%%", remain)))
}

func formatResetHuman(t *time.Time, now time.Time) string {
	if t == nil {
		return "-"
	}
	d := t.Sub(now)
	if d <= 0 {
		return "now"
	}
	return "in " + compactDurationRemain(d.Round(time.Minute))
}

func compactDurationRemain(d time.Duration) string {
	switch {
	case d < time.Hour:
		m := max(1, int(d.Minutes()))
		return fmt.Sprintf("%dm", m)
	default:
		totalH := int(d.Hours())
		if totalH < 24 {
			m := int(d.Minutes()) % 60
			return fmt.Sprintf("%dh %dm", totalH, m)
		}
		days := totalH / 24
		remH := totalH % 24
		if remH == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, remH)
	}
}
