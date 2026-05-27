package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/jyablonski/arc/internal/sysupdate"
)

type linuxManager struct{}

type packageStats struct {
	TotalPackages       int                      `json:"total_packages"`
	ExplicitlyInstalled int                      `json:"explicitly_installed"`
	ForeignPackages     int                      `json:"foreign_packages"`
	TotalInstalledSize  float64                  `json:"total_installed_size_gib"`
	CacheSize           string                   `json:"cache_size"`
	OrphanedPackages    []string                 `json:"orphaned_packages"`
	RecentlyInstalled   int                      `json:"recently_installed"`
	LargestPackages     []map[string]interface{} `json:"largest_packages"`
}

func (linuxManager) UpdateSystem(opts UpdateOptions) error {
	return sysupdate.Run(sysupdate.Options{
		SkipAUR:   opts.SkipAUR,
		SkipCache: opts.SkipCache,
	})
}

func (linuxManager) Clean(opts CleanOptions) error {
	if err := pacman.CheckPacmanAvailable(); err != nil {
		return err
	}

	if !opts.OrphansOnly {
		output.Header("cleaning package cache")
		if _, err := shell.RunSudo("pacman", "-Sc", "--noconfirm"); err != nil {
			return fmt.Errorf("failed to clean cache: %w", err)
		}
		output.Success("Package cache cleaned")
	}

	if !opts.CacheOnly {
		output.Header("removing orphaned packages")
		orphans, err := pacman.GetOrphanedPackages()
		if err != nil {
			return fmt.Errorf("failed to get orphaned packages: %w", err)
		}

		if len(orphans) == 0 {
			output.Info("No orphans to remove")
		} else {
			args := append([]string{"pacman", "-Rns", "--noconfirm"}, orphans...)
			if _, err := shell.RunSudo(args[0], args[1:]...); err != nil {
				output.Warning(fmt.Sprintf("Failed to remove some orphans: %v", err))
			} else {
				output.Success(fmt.Sprintf("Removed %d orphaned packages", len(orphans)))
			}
		}
	}

	return nil
}

func (linuxManager) Installed(opts InstalledOptions) error {
	if err := pacman.CheckPacmanAvailable(); err != nil {
		return err
	}

	var packages []string
	var err error
	if opts.ForeignOnly {
		packages, err = pacman.GetForeignPackages()
		if err != nil {
			return fmt.Errorf("failed to get foreign packages: %w", err)
		}
	} else {
		packages, err = pacman.GetExplicitlyInstalled()
		if err != nil {
			return fmt.Errorf("failed to get installed packages: %w", err)
		}
	}

	if opts.Count {
		fmt.Println(len(packages))
		return nil
	}
	for _, pkg := range packages {
		fmt.Println(pkg)
	}
	return nil
}

func (linuxManager) Packages(opts PackageOptions) error {
	if err := pacman.CheckPacmanAvailable(); err != nil {
		return err
	}

	stats := packageStats{}

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

	orphans, err := pacman.GetOrphanedPackages()
	if err != nil {
		return fmt.Errorf("failed to get orphaned packages: %w", err)
	}
	stats.OrphanedPackages = orphans

	recent, err := pacman.GetRecentlyInstalledCount(opts.Days)
	if err != nil {
		return fmt.Errorf("failed to get recently installed count: %w", err)
	}
	stats.RecentlyInstalled = recent

	largest, err := pacman.GetLargestPackages(opts.Top)
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

	if opts.JSON {
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
	output.Header(fmt.Sprintf("=== Recently Installed (%d days) ===", opts.Days))
	fmt.Printf("%d\n\n", stats.RecentlyInstalled)
	output.Header(fmt.Sprintf("=== Top %d Largest Packages ===", opts.Top))
	rows := make([][]string, len(largest))
	for i, pkg := range largest {
		rows[i] = []string{fmt.Sprintf("%s %s", pkg.Size, pkg.Unit), pkg.Name}
	}
	output.Table([]string{"Size", "Package"}, rows)
	return nil
}
