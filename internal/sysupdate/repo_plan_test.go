package sysupdate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePacmanPlan(t *testing.T) {
	plan, err := parsePacmanPlan(""+
		"linux|1:6.12.1.arch1-1|104857600|\n"+
		"mbedtls3|3.6.7-1|2097152|\n"+
		"bolt|0.9.11-3|524288|\n"+
		"new-tls|4.0-1|1024|old-tls<4 unrelated\n", map[string]string{
		"linux":   "1:6.11.9.arch1-1",
		"bolt":    "0.9.11-2",
		"old-tls": "3.0-1",
	})
	require.NoError(t, err)
	require.Equal(t, []PackageChange{
		{Name: "bolt", FromVersion: "0.9.11-2", ToVersion: "0.9.11-3", SizeBytes: 524288},
		{Name: "linux", FromVersion: "1:6.11.9.arch1-1", ToVersion: "1:6.12.1.arch1-1", SizeBytes: 104857600},
		{Name: "mbedtls3", ToVersion: "3.6.7-1", Note: "new dep", SizeBytes: 2097152},
		{Name: "new-tls", ToVersion: "4.0-1", Note: "replaces old-tls", SizeBytes: 1024, Replaces: "old-tls"},
	}, plan)
}

func TestParsePacmanPlan_rejectsUntrustedShape(t *testing.T) {
	for _, input := range []string{
		"warning from wrapper",
		"foo|1.0|not-a-size|",
		"foo|1.0|1|\nfoo|1.0|1|",
	} {
		_, err := parsePacmanPlan(input, nil)
		require.Error(t, err, input)
	}
}

func TestSamePackagePlan(t *testing.T) {
	a := []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2", SizeBytes: 10}}
	require.True(t, samePackagePlan(a, append([]PackageChange(nil), a...)))
	require.False(t, samePackagePlan(a, []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "3", SizeBytes: 10}}))
}
