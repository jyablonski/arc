package cursor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportFromDashboard_individualPercents(t *testing.T) {
	u := dashboardUsageEnvelope{
		Enabled:           ptrBool(true),
		BillingCycleStart: "1730000000000",
		BillingCycleEnd:   "1732500000000",
		PlanUsage: &planUsage{
			Limit:            floatPtr(40000),
			TotalPercentUsed: floatPtr(27),
			AutoPercentUsed:  floatPtr(12),
			ApiPercentUsed:   floatPtr(75),
			IncludedSpend:    floatPtr(10800),
			Remaining:        floatPtr(29200),
		},
	}
	rep, err := reportFromDashboard(&u, "pro", "Pro")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rep.Windows), 3)
	find := func(label string) (float64, bool) {
		for _, w := range rep.Windows {
			if w.Label == label {
				return w.PercentUsed, true
			}
		}
		return 0, false
	}
	v, ok := find("Total")
	require.True(t, ok)
	require.InDelta(t, 27, v, 1e-6)
	v, ok = find("Auto + Composer")
	require.True(t, ok)
	require.InDelta(t, 12, v, 1e-6)
	v, ok = find("API")
	require.True(t, ok)
	require.InDelta(t, 75, v, 1e-6)
}

func floatPtr(v float64) *float64 { return &v }
func ptrBool(b bool) *bool        { return &b }
