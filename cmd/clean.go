package cmd

import (
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/spf13/cobra"
)

var (
	cleanOrphansOnly bool
	cleanCacheOnly   bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean package cache and remove orphaned packages",
	Long: `Clean package-manager cache and unused dependencies.
Linux uses pacman -Sc and pacman -Rns. macOS uses brew cleanup and brew autoremove.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.PkgMgr.Clean(pkgmgr.CleanOptions{
			OrphansOnly: cleanOrphansOnly,
			CacheOnly:   cleanCacheOnly,
		})
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanOrphansOnly, "orphans-only", false, "Only remove orphaned packages")
	cleanCmd.Flags().BoolVar(&cleanCacheOnly, "cache-only", false, "Only clean package cache")
}
