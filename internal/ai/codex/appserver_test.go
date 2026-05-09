package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
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

func TestDecodeRateLimitsResult_byLimitID(t *testing.T) {
	raw := json.RawMessage(`{
	  "rateLimitsByLimitId": {
	    "a": {
	      "limitId": "a",
	      "primary": { "usedPercent": 5, "windowDurationMins": 300, "resetsAt": 1730947200 }
	    },
	    "b": {
	      "limitId": "b",
	      "primary": { "usedPercent": 7, "windowDurationMins": 300, "resetsAt": 1730947300 }
	    }
	  }
	}`)
	rep, err := decodeRateLimitsResult(raw)
	require.NoError(t, err)
	require.Len(t, rep.Windows, 2)
	require.Equal(t, "5 hour", rep.Windows[0].Label)
	require.InDelta(t, 5.0, rep.Windows[0].PercentUsed, 1e-9)
}

func TestDecodeRateLimitsResult_extraReachedType(t *testing.T) {
	raw := json.RawMessage(`{
	  "rateLimits": {
	    "limitId": "codex",
	    "rateLimitReachedType": "soft",
	    "primary": { "usedPercent": 1, "windowDurationMins": 300, "resetsAt": 1730947200 }
	  }
	}`)
	rep, err := decodeRateLimitsResult(raw)
	require.NoError(t, err)
	require.Equal(t, "soft", rep.Extra["rate_limit_reached_type"])
}

func TestProvider_Usage_fakeCodexAppServer(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "codex")
	scriptBody := `#!/bin/sh
while read -r line; do
  case "$line" in
    *initialize*)
      printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{}}'
      ;;
    *rateLimits*)
      printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":25,"windowDurationMins":300,"resetsAt":1730947200},"secondary":{"usedPercent":10,"windowDurationMins":10080,"resetsAt":1731532800}}}}'
      ;;
  esac
done
`
	require.NoError(t, os.WriteFile(script, []byte(scriptBody), filemode.Executable))

	p := &Provider{CodexBinary: script}
	rep, err := p.Usage(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rep.Windows), 2)
	require.Equal(t, "5 hour", rep.Windows[0].Label)
	require.Equal(t, "weekly", rep.Windows[1].Label)
	require.Equal(t, "codex", p.Name())
}
