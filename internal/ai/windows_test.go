package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeWindow(t *testing.T) {
	for _, tc := range []struct{ provider, label, want string }{
		{"claude", "5 hour", "5 hour"},
		{"claude", "7 day (all models)", "7 day"},
		{"claude", "7 day (Opus)", "7 day · opus"},
		{"codex", "weekly", "7 day"},
		{"codex", "5 hour", "5 hour"},
		{"cursor", "API", "month · api"},
		{"cursor", "Auto + Composer", "month · auto"},
		{"cursor", "Total", "month · total"},
		{"cursor", "gpt-4 requests", "month · gpt-4 requests"},
	} {
		require.Equal(t, tc.want, NormalizeWindow(tc.provider, tc.label), tc.label)
	}
}

func TestNormalizeWindows_keepsProviderLabel(t *testing.T) {
	agg := AggregateReport{Providers: []ProviderResult{{Name: "codex", Report: UsageReport{Windows: []UsageWindow{{Label: "weekly"}}}}}}
	NormalizeWindows(&agg)
	w := agg.Providers[0].Report.Windows[0]
	require.Equal(t, "weekly", w.Label)
	require.Equal(t, "7 day", w.Window)
}
