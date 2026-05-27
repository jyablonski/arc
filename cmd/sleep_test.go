package cmd

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestSleepCmd_runsSuspend(t *testing.T) {
	isolateCommandTreeExtras(t)
	shell.SetMockRunner(&shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			if name == "sudo" && len(args) >= 2 && args[0] == "systemctl" && args[1] == "suspend" {
				return "", nil
			}
			return "", fmt.Errorf("unexpected %s %v", name, args)
		},
	})
	t.Cleanup(shell.ClearMockRunner)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"sleep"})
	require.NoError(t, rootCmd.Execute())
}

func TestSleepCmd_darwinUsesPmset(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	shell.SetMockRunner(&shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "pmset", name)
			require.Equal(t, []string{"sleepnow"}, args)
			return "", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, sleepCmd.RunE(sleepCmd, []string{}))
}
