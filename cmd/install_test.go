package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runInstallCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetInstallFlags(t)
	resetJSONFlag(t)
	rootCmd.SetArgs(args)
	var cobraOut bytes.Buffer
	rootCmd.SetOut(&cobraOut)
	rootCmd.SetErr(&cobraOut)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	var err error
	out := captureStdout(t, func() { err = rootCmd.Execute() })
	return out + cobraOut.String(), err
}

func resetInstallFlags(t *testing.T) {
	t.Helper()
	reset := func() {
		for _, name := range []string{"aur", "cask", "formula", "help"} {
			if flag := installCmd.Flags().Lookup(name); flag != nil {
				require.NoError(t, flag.Value.Set("false"))
				flag.Changed = false
			}
		}
	}
	reset()
	t.Cleanup(reset)
}

func TestInstallCmd_dispatch(t *testing.T) {
	tests := []struct {
		name string
		os   platform.OS
		args []string
		want pkgmgr.InstallOptions
	}{
		{"linux default", platform.Linux, []string{"install", "ripgrep", "fd"}, pkgmgr.InstallOptions{Packages: []string{"ripgrep", "fd"}}},
		{"linux aur after package", platform.Linux, []string{"install", "some-package", "--aur"}, pkgmgr.InstallOptions{Packages: []string{"some-package"}, AUR: true}},
		{"linux aur before packages", platform.Linux, []string{"install", "--aur", "first", "second"}, pkgmgr.InstallOptions{Packages: []string{"first", "second"}, AUR: true}},
		{"macos default", platform.Darwin, []string{"install", "ripgrep", "fd"}, pkgmgr.InstallOptions{Packages: []string{"ripgrep", "fd"}}},
		{"macos cask", platform.Darwin, []string{"install", "some-app", "--cask"}, pkgmgr.InstallOptions{Packages: []string{"some-app"}, Cask: true}},
		{"macos formula", platform.Darwin, []string{"install", "--formula", "ripgrep"}, pkgmgr.InstallOptions{Packages: []string{"ripgrep"}, Formula: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return nil }}
			t.Cleanup(setAppForTest(&App{Platform: tt.os, PkgMgr: mgr}))
			_, err := runInstallCommand(t, tt.args...)
			require.NoError(t, err)
			require.Len(t, mgr.InstallCalls(), 1)
			assert.Equal(t, tt.want, mgr.InstallCalls()[0].Opts)
		})
	}
}

func TestInstallCmd_rejectsUnsupportedOptionsBeforeManager(t *testing.T) {
	tests := []struct {
		name string
		os   platform.OS
		args []string
		want error
	}{
		{"aur on macos", platform.Darwin, []string{"install", "x", "--aur"}, arcerrs.ErrAURLinuxOnly},
		{"explicit false aur on macos", platform.Darwin, []string{"install", "x", "--aur=false"}, arcerrs.ErrAURLinuxOnly},
		{"cask on linux", platform.Linux, []string{"install", "x", "--cask"}, arcerrs.ErrCaskDarwinOnly},
		{"formula on linux", platform.Linux, []string{"install", "x", "--formula"}, arcerrs.ErrFormulaDarwinOnly},
		{"json on linux", platform.Linux, []string{"install", "x", "--json"}, arcerrs.ErrInstallJSONUnsupported},
		{"json on macos", platform.Darwin, []string{"--json", "install", "x"}, arcerrs.ErrInstallJSONUnsupported},
		{"short json", platform.Linux, []string{"install", "x", "-j"}, arcerrs.ErrInstallJSONUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return nil }}
			t.Cleanup(setAppForTest(&App{Platform: tt.os, PkgMgr: mgr}))
			out, err := runInstallCommand(t, tt.args...)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, mgr.InstallCalls())
			require.Empty(t, out)
		})
	}
}

func TestInstallCmd_rejectsMissingPackages(t *testing.T) {
	for _, args := range [][]string{
		{"install"},
		{"install", "--aur"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return nil }}
			t.Cleanup(setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr}))
			_, err := runInstallCommand(t, args...)
			require.Error(t, err)
			require.Empty(t, mgr.InstallCalls())
		})
	}
}

func TestInstallCmd_rejectsConflictingSources(t *testing.T) {
	for _, tt := range []struct {
		os   platform.OS
		args []string
	}{
		{platform.Linux, []string{"install", "x", "--aur", "--cask"}},
		{platform.Darwin, []string{"install", "x", "--cask", "--formula"}},
		{platform.Linux, []string{"install", "x", "--aur", "--formula"}},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return nil }}
			t.Cleanup(setAppForTest(&App{Platform: tt.os, PkgMgr: mgr}))
			_, err := runInstallCommand(t, tt.args...)
			require.ErrorContains(t, err, "if any flags in the group [aur cask formula] are set none of the others can be")
			require.Empty(t, mgr.InstallCalls())
		})
	}
}

func TestInstallCmd_failureDoesNotRetryOrSwitchSource(t *testing.T) {
	want := errors.New("package not found")
	mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return want }}
	t.Cleanup(setAppForTest(&App{Platform: platform.Linux, PkgMgr: mgr}))
	out, err := runInstallCommand(t, "install", "missing-package")
	require.ErrorIs(t, err, want)
	require.Len(t, mgr.InstallCalls(), 1)
	require.False(t, mgr.InstallCalls()[0].Opts.AUR)
	require.NotContains(t, out, "install complete")
}

func TestInstallCmd_helpDescribesDetectedPlatform(t *testing.T) {
	tests := []struct {
		name string
		os   platform.OS
		want []string
	}{
		{"linux", platform.Linux, []string{"Detected platform: Linux.", "Default: configured pacman repositories", "sudo pacman -S --needed", "yay -S --aur --needed", "arc update system"}},
		{"macos", platform.Darwin, []string{"Detected platform: macOS.", "Default: Homebrew", "brew install <package>", "brew install --cask", "brew install --formula"}},
		{"unknown", platform.Unknown, []string{"Detected platform: unknown.", "Default: none."}},
	}
	for _, tt := range tests {
		for _, args := range [][]string{{"install", "--help"}, {"help", "install"}} {
			t.Run(tt.name+"/"+args[0], func(t *testing.T) {
				mgr := &pkgmgr.ManagerMock{InstallFunc: func(pkgmgr.InstallOptions) error { return nil }}
				t.Cleanup(setAppForTest(&App{Platform: tt.os, PkgMgr: mgr}))
				out, err := runInstallCommand(t, args...)
				require.NoError(t, err)
				for _, want := range tt.want {
					require.Contains(t, out, want)
				}
				require.Contains(t, out, "it never switches sources or guesses a name")
				require.Contains(t, out, "does not support --json")
				require.Empty(t, mgr.InstallCalls())
			})
		}
	}
}
