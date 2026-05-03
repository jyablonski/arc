package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHumanizeBucketKey_omeletteAlias(t *testing.T) {
	require.Equal(t, "7 day (Claude Design)", humanizeBucketKey("seven_day_omelette"))
	require.Equal(t, "7 day (Claude Design)", humanizeBucketKey("seven_day_Omelette"))
}
