package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestPackagesCmd_pacmanMissing(t *testing.T) {
	defer setAppForTest(newApp(platform.Linux))()

	isolateCommandTreeExtras(t)
	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name != "pacman" },
	})
	t.Cleanup(shell.ClearMockRunner)
	defer func() { rootCmd.SetArgs(nil) }()

	rootCmd.SetArgs([]string{"packages"})
	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestPackagesCmd_darwinJSON(t *testing.T) {
	defer setAppForTest(newApp(platform.Darwin))()
	packagesJSON = true
	t.Cleanup(func() { packagesJSON = false })

	shell.SetMockRunner(&shell.MockRunner{
		CommandExistsFunc: func(name string) bool { return name == "brew" },
		RunFunc: func(name string, args ...string) (string, error) {
			switch {
			case name == "brew" && len(args) == 2 && args[0] == "list" && args[1] == "--formula":
				return "git\nuv", nil
			case name == "brew" && len(args) == 2 && args[0] == "list" && args[1] == "--cask":
				return "cursor", nil
			case name == "brew" && len(args) == 1 && args[0] == "leaves":
				return "git", nil
			case name == "brew" && len(args) == 1 && args[0] == "--cache":
				return "/tmp/homebrew-cache", nil
			case name == "du":
				return "10M\t/tmp/homebrew-cache", nil
			default:
				return "", nil
			}
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	output := captureStdout(t, func() {
		require.NoError(t, packagesCmd.RunE(packagesCmd, []string{}))
	})
	require.Contains(t, output, `"platform": "darwin"`)
	require.Contains(t, output, `"formulae": 2`)
	require.Contains(t, output, `"casks": 1`)
}
