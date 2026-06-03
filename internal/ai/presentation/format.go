package presentation

import (
	"fmt"
	"math"
	"strings"
)

const dash = "—"

func humanizeCount(v int64) string {
	if v < 0 {
		v = 0
	}
	type unit struct {
		value  float64
		suffix string
	}
	units := []unit{
		{1_000_000_000, "B"},
		{1_000_000, "M"},
		{1_000, "K"},
	}
	for _, u := range units {
		if float64(v) >= u.value {
			scaled := roundHalfUp(float64(v)/u.value, 1)
			return fmt.Sprintf("%.1f%s", scaled, u.suffix)
		}
	}
	return fmt.Sprintf("%d", v)
}

func formatCurrency(v float64, total bool, adaptivePrecision bool) string {
	if v < 0 {
		v = 0
	}
	if total || !adaptivePrecision || v >= 1 {
		return fmt.Sprintf("$%.2f", roundHalfUp(v, 2))
	}
	return fmt.Sprintf("$%.4f", roundHalfUp(v, 4))
}

func computeShare(rowCost, totalCost float64) float64 {
	if totalCost <= 0 {
		return 0
	}
	return 100 * rowCost / totalCost
}

func formatShare(v float64) string {
	return fmt.Sprintf("%.1f%%", roundHalfUp(v, 1))
}

func roundHalfUp(v float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Floor(v*scale+0.5) / scale
}

func alignDecimal(values []string) []string {
	maxLeft := 0
	for _, v := range values {
		left := decimalLeftLen(v)
		if left > maxLeft {
			maxLeft = left
		}
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.Repeat(" ", maxLeft-decimalLeftLen(v)) + v
	}
	return out
}

func decimalLeftLen(v string) int {
	idx := strings.IndexByte(v, '.')
	if idx < 0 {
		idx = strings.IndexByte(v, '%')
	}
	if idx < 0 {
		return len(v)
	}
	return idx
}

func shortSessionID(id string) string {
	for _, part := range strings.Split(id, "-") {
		if len(part) == 8 && isLowerHex(part) {
			return part
		}
	}
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func isLowerHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return s != ""
}
