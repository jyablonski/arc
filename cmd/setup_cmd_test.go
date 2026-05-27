package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestSetupCmd_pacmanMissing(t *testing.T) {
	defer setAppForTest(newApp(platform.Linux))()

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

func TestSetupCmd_darwinBrewMissing(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "brew" },
	})
	t.Cleanup(shell.ClearMockRunner)

	err := setupCmd.RunE(setupCmd, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "brew is not available")
}

func TestSetupCmd_darwinInstallsMissingTools(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	var calls [][]string
	available := map[string]bool{"brew": true, "git": true}
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return available[name] },
		RunInteractiveFunc: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, setupCmd.RunE(setupCmd, []string{}))
	require.Equal(t, [][]string{
		{"brew", "install", "gh"},
		{"brew", "install", "fastfetch"},
		{"brew", "install", "uv"},
	}, calls)
}
