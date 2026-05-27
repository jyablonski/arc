package cmd

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/platform"
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

func TestUpdateSystemCmd_darwinRunsBrew(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	updateNoCache = false
	t.Cleanup(func() { updateNoCache = false })

	var calls [][]string
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "brew" },
		RunInteractiveFunc: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, updateSystemCmd.RunE(updateSystemCmd, []string{}))
	require.Equal(t, [][]string{
		{"brew", "update"},
		{"brew", "upgrade"},
		{"brew", "cleanup"},
	}, calls)
}

func TestUpdateSystemCmd_darwinNoAURUnsupported(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	flag := updateSystemCmd.Flags().Lookup("no-aur")
	oldChanged := flag.Changed
	oldNoAUR := updateNoAUR
	flag.Changed = true
	updateNoAUR = true
	t.Cleanup(func() {
		flag.Changed = oldChanged
		updateNoAUR = oldNoAUR
	})

	err := updateSystemCmd.RunE(updateSystemCmd, []string{})
	require.Error(t, err)
	require.Contains(t, fmt.Sprint(err), "--no-aur is only supported on Linux")
}
