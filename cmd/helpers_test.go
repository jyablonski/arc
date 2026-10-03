package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetJSONFlag clears the global -j flag now and when the test ends. The flag
// lives on the shared rootCmd, so a test that passes -j would otherwise leave
// it set for whichever test runs next.
func resetJSONFlag(t *testing.T) {
	t.Helper()
	reset := func() { require.NoError(t, rootCmd.PersistentFlags().Set("json", "false")) }
	reset()
	t.Cleanup(reset)
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}
