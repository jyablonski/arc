package setupdeps

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	require.IsType(t, linuxInstaller{}, New(platform.Linux))
	require.IsType(t, darwinInstaller{}, New(platform.Darwin))
	require.IsType(t, unsupportedInstaller{}, New(platform.Unknown))
}

func TestUnsupportedInstaller(t *testing.T) {
	err := New(platform.Unknown).Install()
	require.True(t, errors.Is(err, ErrUnsupportedPlatform))
}
