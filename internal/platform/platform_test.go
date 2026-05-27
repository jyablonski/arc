package platform

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	require.Equal(t, Parse(runtime.GOOS), Detect())
}

func TestString(t *testing.T) {
	require.Equal(t, "linux", Linux.String())
	require.Equal(t, "darwin", Darwin.String())
	require.Equal(t, "unknown", Unknown.String())
}
