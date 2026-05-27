package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/spf13/cobra"
)

var (
	installedAUROnly bool
	installedCount   bool
)

var installedCmd = &cobra.Command{
	Use:   "installed",
	Short: "List explicitly installed packages",
	Long: `List packages that were explicitly installed (not as dependencies).
Linux uses pacman -Qe. macOS lists installed Homebrew formulae.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if installedAUROnly && app.Platform != platform.Linux {
			return fmt.Errorf("--aur-only is only supported on Linux")
		}
		return app.PkgMgr.Installed(pkgmgr.InstalledOptions{
			ForeignOnly: installedAUROnly,
			Count:       installedCount,
		})
	},
}

func init() {
	rootCmd.AddCommand(installedCmd)
	installedCmd.Flags().BoolVar(&installedAUROnly, "aur-only", false, "Show only AUR packages on Linux")
	installedCmd.Flags().BoolVar(&installedCount, "count", false, "Show only the count")
}
