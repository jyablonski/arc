package presentation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompactDurationRemain(t *testing.T) {
	require.Equal(t, "30m", compactDurationRemain(30*time.Minute))
	require.Equal(t, "5h 0m", compactDurationRemain(5*time.Hour))
	require.Equal(t, "5h 30m", compactDurationRemain(5*time.Hour+30*time.Minute))
	require.Equal(t, "2d 3h", compactDurationRemain(51*time.Hour))
	require.Equal(t, "12d", compactDurationRemain(12*24*time.Hour))
}

func TestPctRemainingForDisplay(t *testing.T) {
	require.Equal(t, -1.0, pctRemainingForDisplay(-1))
	require.InDelta(t, 100, pctRemainingForDisplay(0), 0.001)
	require.InDelta(t, 72, pctRemainingForDisplay(28), 0.001)
	require.InDelta(t, 24.6, pctRemainingForDisplay(75.4), 0.05)
	require.InDelta(t, 0, pctRemainingForDisplay(100), 0.001)
	require.InDelta(t, 0, pctRemainingForDisplay(150), 0.001)
}
