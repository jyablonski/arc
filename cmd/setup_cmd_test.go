package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestSetupCmd_pacmanMissing(t *testing.T) {
	isolateCommandTreeExtras(t)
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "pacman" },
	})
	t.Cleanup(shell.ClearMockRunner)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"setup"})
	err := rootCmd.Execute()
	require.Error(t, err)
}
