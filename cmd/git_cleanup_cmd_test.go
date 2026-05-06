package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestGitCleanupCmd_gitMissing(t *testing.T) {
	isolateCommandTreeExtras(t)
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "git" },
	})
	t.Cleanup(shell.ClearMockRunner)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"git", "cleanup"})
	err := rootCmd.Execute()
	require.Error(t, err)
}
