package cmd

import (
	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/spf13/cobra"
)

var (
	installAUR     bool
	installCask    bool
	installFormula bool
)

var installCmd = &cobra.Command{
	Use:   "install <package>...",
	Short: "Install packages with pacman, yay, or Homebrew",
	Long:  installHelp(app.Platform),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			return arcerrs.ErrInstallJSONUnsupported
		}
		if cmd.Flags().Changed("aur") && app.Platform != platform.Linux {
			return arcerrs.ErrAURLinuxOnly
		}
		if cmd.Flags().Changed("cask") && app.Platform != platform.Darwin {
			return arcerrs.ErrCaskDarwinOnly
		}
		if cmd.Flags().Changed("formula") && app.Platform != platform.Darwin {
			return arcerrs.ErrFormulaDarwinOnly
		}

		output.Title("arc install", app.Platform.String())
		if err := app.PkgMgr.Install(pkgmgr.InstallOptions{
			Packages: args,
			AUR:      installAUR,
			Cask:     installCask,
			Formula:  installFormula,
		}); err != nil {
			return err
		}
		output.Summary(output.GlyphOK, "install complete")
		return nil
	},
}

func installHelp(os platform.OS) string {
	help := "Install one or more packages using the host package manager.\n\n"
	switch os {
	case platform.Linux:
		help += `Detected platform: Linux.
Default: configured pacman repositories (Arch Linux), using:
  sudo pacman -S --needed <package>...

Use --aur to select AUR targets explicitly:
  yay -S --aur --needed <package>...
Repository dependencies are still allowed. yay runs as your user and keeps
its configured build-file review and confirmation prompts.

--cask and --formula are only supported on macOS.

System upgrades are separate. arc does not refresh pacman's databases or
perform a full system upgrade. If the databases are stale, run
arc update system first, then retry the install.
`
	case platform.Darwin:
		help += `Detected platform: macOS.
Default: Homebrew, using:
  brew install <package>...
Homebrew resolves names as formulae or casks. Select a type explicitly with:
  --cask       brew install --cask <package>...
  --formula    brew install --formula <package>...

--aur is only supported on Linux. Homebrew keeps its normal dependency,
update, cleanup, and confirmation behavior.
`
	default:
		help += `Detected platform: unknown.
Default: none. Package installation is only supported on Arch Linux
(pacman/yay) and macOS (Homebrew).
`
	}
	return help + `
Use package names from the selected package manager. If a package is not
found, arc returns an error; it never switches sources or guesses a name.
--aur, --cask, and --formula are mutually exclusive.

This interactive command does not support --json.`
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVar(&installAUR, "aur", false, "Install requested targets from the AUR (Linux only)")
	installCmd.Flags().BoolVar(&installCask, "cask", false, "Install Homebrew casks (macOS only)")
	installCmd.Flags().BoolVar(&installFormula, "formula", false, "Install Homebrew formulae (macOS only)")
	installCmd.MarkFlagsMutuallyExclusive("aur", "cask", "formula")

	help := installCmd.HelpFunc()
	installCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.Long = installHelp(app.Platform)
		help(cmd, args)
	})
}
