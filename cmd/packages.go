package cmd

import (
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/spf13/cobra"
)

var (
	packagesDays int
	packagesTop  int
	packagesJSON bool
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Show package statistics and information",
	Long: `Display package statistics including counts, sizes, cache information,
and platform-specific package-manager details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.PkgMgr.Packages(pkgmgr.PackageOptions{
			Days: packagesDays,
			Top:  packagesTop,
			JSON: packagesJSON,
		})
	},
}

func init() {
	rootCmd.AddCommand(packagesCmd)
	packagesCmd.Flags().IntVar(&packagesDays, "days", 7, "Days to look back for recently installed packages")
	packagesCmd.Flags().IntVar(&packagesTop, "top", 25, "Number of largest packages to show")
	packagesCmd.Flags().BoolVar(&packagesJSON, "json", false, "Output in JSON format")
}
