package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestUpdateSelfCmd_runs(t *testing.T) {
	isolateCommandTreeExtras(t)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"update", "self"})
	require.NoError(t, rootCmd.Execute())
}

func TestUpdateUvCmd_uvMissing(t *testing.T) {
	isolateCommandTreeExtras(t)
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "uv" },
	})
	t.Cleanup(shell.ClearMockRunner)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"update", "uv"})
	err := rootCmd.Execute()
	require.Error(t, err)
}
