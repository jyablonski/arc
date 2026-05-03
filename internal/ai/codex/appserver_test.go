package codex

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeRateLimitsResult(t *testing.T) {
	raw := json.RawMessage(`{
	  "rateLimits": {
	    "limitId": "codex",
	    "primary": { "usedPercent": 25, "windowDurationMins": 300, "resetsAt": 1730947200 },
	    "secondary": { "usedPercent": 10, "windowDurationMins": 10080, "resetsAt": 1731532800 }
	  }
	}`)
	rep, err := decodeRateLimitsResult(raw)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rep.Windows), 2)
	require.Equal(t, "5 hour", rep.Windows[0].Label)
	require.Equal(t, "weekly", rep.Windows[1].Label)
}
