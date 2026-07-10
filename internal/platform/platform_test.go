package platform

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		require.Equal(t, Linux, Detect())
	case "darwin":
		require.Equal(t, Darwin, Detect())
	default:
		require.Equal(t, Unknown, Detect())
	}
}

func TestString(t *testing.T) {
	require.Equal(t, "linux", Linux.String())
	require.Equal(t, "darwin", Darwin.String())
	require.Equal(t, "unknown", Unknown.String())
}
