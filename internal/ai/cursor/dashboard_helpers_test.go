package cursor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinitePositive_finiteNumber(t *testing.T) {
	require.False(t, finitePositive(nil))
	neg := -1.0
	require.False(t, finitePositive(&neg))
	z := 0.0
	require.False(t, finitePositive(&z))
	ok := 3.14
	require.True(t, finitePositive(&ok))
}

func TestFiniteNumber(t *testing.T) {
	require.False(t, finiteNumber(nil))
	neg := -1.0
	require.True(t, finiteNumber(&neg))
	z := 0.0
	require.True(t, finiteNumber(&z))
}

func TestShouldUseEnterpriseTeamREST(t *testing.T) {
	tr := true
	limit := 100.0
	t.Run("enterprise_missing_plan_usage", func(t *testing.T) {
		u := &dashboardUsageEnvelope{Enabled: &tr}
		require.True(t, shouldUseEnterpriseTeamREST(u, "enterprise"))
	})
	t.Run("team_limit_missing", func(t *testing.T) {
		u := &dashboardUsageEnvelope{
			Enabled:   &tr,
			PlanUsage: &planUsage{Limit: nil},
		}
		require.True(t, shouldUseEnterpriseTeamREST(u, "team"))
	})
	t.Run("disabled", func(t *testing.T) {
		f := false
		u := &dashboardUsageEnvelope{Enabled: &f}
		require.False(t, shouldUseEnterpriseTeamREST(u, "enterprise"))
	})
	t.Run("has_limit", func(t *testing.T) {
		u := &dashboardUsageEnvelope{
			Enabled:   &tr,
			PlanUsage: &planUsage{Limit: &limit},
		}
		require.False(t, shouldUseEnterpriseTeamREST(u, "enterprise"))
	})
}

func TestShouldUseUnknownREST(t *testing.T) {
	tr := true
	u := &dashboardUsageEnvelope{Enabled: &tr, PlanUsage: &planUsage{}}
	require.True(t, shouldUseUnknownREST(u, "", true))
	require.False(t, shouldUseUnknownREST(u, "pro", true))
	f := false
	u.Enabled = &f
	require.False(t, shouldUseUnknownREST(u, "", true))
}

func TestShouldUseRESTLimitMissingNoTotal(t *testing.T) {
	tr := true
	u := &dashboardUsageEnvelope{
		Enabled:   &tr,
		PlanUsage: &planUsage{},
	}
	require.True(t, shouldUseRESTLimitMissingNoTotal(u, "pro"))
	require.False(t, shouldUseRESTLimitMissingNoTotal(u, ""))
	require.False(t, shouldUseRESTLimitMissingNoTotal(u, "  "))
}

func TestShouldUseRESTTeamNoLimit(t *testing.T) {
	tr := true
	u := &dashboardUsageEnvelope{
		Enabled: &tr,
		SpendLimitUsage: &spendLimitUsage{
			LimitType: ptrStr("team"),
			PooledLimit: func() *float64 {
				v := 1.0
				return &v
			}(),
		},
	}
	require.True(t, shouldUseRESTTeamNoLimit(u, "team"))
	require.False(t, shouldUseRESTTeamNoLimit(u, "enterprise"))
}

func TestTeamSignals(t *testing.T) {
	require.True(t, teamSignals(&dashboardUsageEnvelope{}, "team"))
	u := &dashboardUsageEnvelope{
		SpendLimitUsage: &spendLimitUsage{LimitType: ptrStr("team")},
	}
	require.True(t, teamSignals(u, "pro"))
}

func ptrStr(s string) *string { return &s }
