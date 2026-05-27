package hardware

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	require.IsType(t, linuxReporter{}, New(platform.Linux))
	require.IsType(t, darwinReporter{}, New(platform.Darwin))
	require.IsType(t, unsupportedReporter{}, New(platform.Unknown))
}

func TestUnsupportedReporter(t *testing.T) {
	err := New(platform.Unknown).Show([]string{"cpu"})
	require.True(t, errors.Is(err, ErrUnsupportedPlatform))
}
