package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/spf13/cobra"
)

var (
	packagesDays int
	packagesTop  int
	packagesJSON bool
)

type PackageStats struct {
	TotalPackages       int                      `json:"total_packages"`
	ExplicitlyInstalled int                      `json:"explicitly_installed"`
	ForeignPackages     int                      `json:"foreign_packages"`
	TotalInstalledSize  float64                  `json:"total_installed_size_gib"`
	CacheSize           string                   `json:"cache_size"`
	OrphanedPackages    []string                 `json:"orphaned_packages"`
	RecentlyInstalled   int                      `json:"recently_installed"`
	LargestPackages     []map[string]interface{} `json:"largest_packages"`
}

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Show package statistics and information",
	Long: `Display package statistics including counts, sizes, cache information,
orphaned packages, recently installed packages, and largest packages.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		stats := PackageStats{}

		// Get package counts
		total, err := pacman.GetPackageCount()
		if err != nil {
			return fmt.Errorf("failed to get package count: %w", err)
		}
		stats.TotalPackages = total

		explicit, err := pacman.GetExplicitlyInstalledCount()
		if err != nil {
			return fmt.Errorf("failed to get explicitly installed count: %w", err)
		}
		stats.ExplicitlyInstalled = explicit

		foreign, err := pacman.GetForeignPackageCount()
		if err != nil {
			return fmt.Errorf("failed to get foreign package count: %w", err)
		}
		stats.ForeignPackages = foreign

		// Get sizes
		totalSize, err := pacman.GetTotalInstalledSize()
		if err != nil {
			return fmt.Errorf("failed to get total installed size: %w", err)
		}
		stats.TotalInstalledSize = totalSize

		cacheSize, err := pacman.GetCacheSize()
		if err != nil {
			stats.CacheSize = "N/A"
		} else {
			stats.CacheSize = cacheSize
		}

		// Get orphaned packages
		orphans, err := pacman.GetOrphanedPackages()
		if err != nil {
			return fmt.Errorf("failed to get orphaned packages: %w", err)
		}
		stats.OrphanedPackages = orphans

		// Get recently installed
		recent, err := pacman.GetRecentlyInstalledCount(packagesDays)
		if err != nil {
			return fmt.Errorf("failed to get recently installed count: %w", err)
		}
		stats.RecentlyInstalled = recent

		// Get largest packages
		largest, err := pacman.GetLargestPackages(packagesTop)
		if err != nil {
			return fmt.Errorf("failed to get largest packages: %w", err)
		}
		stats.LargestPackages = make([]map[string]interface{}, len(largest))
		for i, pkg := range largest {
			stats.LargestPackages[i] = map[string]interface{}{
				"name": pkg.Name,
				"size": fmt.Sprintf("%s %s", pkg.Size, pkg.Unit),
			}
		}

		// Output
		if packagesJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(stats)
		}

		output.Header("=== Package Count ===")
		fmt.Printf("%d\n\n", stats.TotalPackages)

		output.Header("=== Explicitly Installed ===")
		fmt.Printf("%d\n\n", stats.ExplicitlyInstalled)

		output.Header("=== Foreign/AUR Packages ===")
		fmt.Printf("%d\n\n", stats.ForeignPackages)

		output.Header("=== Total Installed Size ===")
		fmt.Printf("%.2f GiB\n\n", stats.TotalInstalledSize)

		output.Header("=== Package Cache Size ===")
		fmt.Printf("%s\n\n", stats.CacheSize)

		output.Header("=== Orphaned Packages ===")
		if len(stats.OrphanedPackages) == 0 {
			fmt.Println("None")
		} else {
			for _, pkg := range stats.OrphanedPackages {
				fmt.Println(pkg)
			}
		}
		fmt.Println()

		output.Header(fmt.Sprintf("=== Recently Installed (%d days) ===", packagesDays))
		fmt.Printf("%d\n\n", stats.RecentlyInstalled)

		output.Header(fmt.Sprintf("=== Top %d Largest Packages ===", packagesTop))
		headers := []string{"Size", "Package"}
		rows := make([][]string, len(largest))
		for i, pkg := range largest {
			rows[i] = []string{fmt.Sprintf("%s %s", pkg.Size, pkg.Unit), pkg.Name}
		}
		output.Table(headers, rows)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(packagesCmd)
	packagesCmd.Flags().IntVar(&packagesDays, "days", 7, "Days to look back for recently installed packages")
	packagesCmd.Flags().IntVar(&packagesTop, "top", 25, "Number of largest packages to show")
	packagesCmd.Flags().BoolVar(&packagesJSON, "json", false, "Output in JSON format")
}
