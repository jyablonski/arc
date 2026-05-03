package cursor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportFromUsageBody(t *testing.T) {
	body := []byte(`{
	  "startOfMonth": "2026-05-01T00:00:00Z",
	  "gpt-4": {"numRequests": 10, "maxRequestUsage": 500}
	}`)
	rep, err := reportFromUsageBody(body)
	require.NoError(t, err)
	require.Len(t, rep.Windows, 1)
	require.Equal(t, "gpt-4 requests", rep.Windows[0].Label)
	require.InDelta(t, 2.0, rep.Windows[0].PercentUsed, 1e-9)
}
