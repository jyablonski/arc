package sysupdate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKernelChangeMessage(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		before := map[string]string{"linux": "1:6.6.1-1"}
		after := map[string]string{"linux": "1:6.6.1-1"}
		ok, msg := kernelChangeMessage(before, after)
		require.False(t, ok)
		require.Empty(t, msg)
	})
	t.Run("version bump", func(t *testing.T) {
		before := map[string]string{"linux": "1:6.6.1-1"}
		after := map[string]string{"linux": "1:6.6.2-1"}
		ok, msg := kernelChangeMessage(before, after)
		require.True(t, ok)
		require.Contains(t, msg, "linux")
		require.Contains(t, msg, "6.6.1-1")
		require.Contains(t, msg, "6.6.2-1")
	})
	t.Run("new kernel package", func(t *testing.T) {
		before := map[string]string{}
		after := map[string]string{"linux-lts": "1:6.1.1-1"}
		ok, msg := kernelChangeMessage(before, after)
		require.True(t, ok)
		require.Contains(t, msg, "New kernel package")
		require.Contains(t, msg, "linux-lts")
	})
}
