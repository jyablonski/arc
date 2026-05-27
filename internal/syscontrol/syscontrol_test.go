package syscontrol

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	require.IsType(t, linuxController{}, New(platform.Linux))
	require.IsType(t, darwinController{}, New(platform.Darwin))
	require.IsType(t, unsupportedController{}, New(platform.Unknown))
}

func TestUnsupportedController(t *testing.T) {
	err := New(platform.Unknown).Sleep()
	require.True(t, errors.Is(err, ErrUnsupportedPlatform))
}
