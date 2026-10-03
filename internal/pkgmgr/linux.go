package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/sysupdate"
)

type linuxManager struct {
	pac pacmanOps
}

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
		AssumeYes: opts.AssumeYes,
		Log:       opts.Log,
		Verbose:   opts.Verbose,
		ShowDiff:  opts.ShowDiff,
	})
}

func (m linuxManager) Clean(opts CleanOptions) error {
	if err := m.pac.CheckPacmanAvailable(); err != nil {
		return err
	}

	if !opts.OrphansOnly {
		if _, err := run.RunSudo("pacman", "-Sc", "--noconfirm"); err != nil {
			return fmt.Errorf("failed to clean cache: %w", err)
		}
		output.Success("package cache cleaned")
	}

	if !opts.CacheOnly {
		orphans, err := m.pac.GetOrphanedPackages()
		if err != nil {
			return fmt.Errorf("failed to get orphaned packages: %w", err)
		}

		if len(orphans) == 0 {
			output.Info("no orphaned packages to remove")
		} else {
			args := append([]string{"pacman", "-Rns", "--noconfirm"}, orphans...)
			if _, err := run.RunSudo(args[0], args[1:]...); err != nil {
				output.Warning(fmt.Sprintf("some orphaned packages not removed: %v", err))
			} else {
				output.Success(fmt.Sprintf("removed %s", output.Count(len(orphans), "orphaned package", "orphaned packages")))
			}
		}
	}

	return nil
}

func (m linuxManager) Installed(opts InstalledOptions) error {
	if err := m.pac.CheckPacmanAvailable(); err != nil {
		return err
	}

	var packages []string
	var err error
	if opts.ForeignOnly {
		packages, err = m.pac.GetForeignPackages()
		if err != nil {
			return fmt.Errorf("failed to get foreign packages: %w", err)
		}
	} else {
		packages, err = m.pac.GetExplicitlyInstalled()
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

func (m linuxManager) Packages(opts PackageOptions) error {
	if err := m.pac.CheckPacmanAvailable(); err != nil {
		return err
	}

	stats := packageStats{}

	total, err := m.pac.GetPackageCount()
	if err != nil {
		return fmt.Errorf("failed to get package count: %w", err)
	}
	stats.TotalPackages = total

	explicit, err := m.pac.GetExplicitlyInstalledCount()
	if err != nil {
		return fmt.Errorf("failed to get explicitly installed count: %w", err)
	}
	stats.ExplicitlyInstalled = explicit

	foreign, err := m.pac.GetForeignPackageCount()
	if err != nil {
		return fmt.Errorf("failed to get foreign package count: %w", err)
	}
	stats.ForeignPackages = foreign

	totalSize, err := m.pac.GetTotalInstalledSize()
	if err != nil {
		return fmt.Errorf("failed to get total installed size: %w", err)
	}
	stats.TotalInstalledSize = totalSize

	cacheSize, err := m.pac.GetCacheSize()
	if err != nil {
		stats.CacheSize = "N/A"
	} else {
		stats.CacheSize = cacheSize
	}

	orphans, err := m.pac.GetOrphanedPackages()
	if err != nil {
		return fmt.Errorf("failed to get orphaned packages: %w", err)
	}
	stats.OrphanedPackages = orphans

	recent, err := m.pac.GetRecentlyInstalledCount(opts.Days)
	if err != nil {
		return fmt.Errorf("failed to get recently installed count: %w", err)
	}
	stats.RecentlyInstalled = recent

	largest, err := m.pac.GetLargestPackages(opts.Top)
	if err != nil {
		return fmt.Errorf("failed to get largest packages: %w", err)
	}
	if opts.JSON {
		stats.LargestPackages = make([]map[string]interface{}, len(largest))
		for i, pkg := range largest {
			stats.LargestPackages[i] = map[string]interface{}{
				"name": pkg.Name,
				"size": fmt.Sprintf("%s %s", pkg.Size, pkg.Unit),
			}
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)
	}

	style := output.StyleFor(os.Stdout)
	sc := &output.Screen{W: os.Stdout, Style: style, Title: "arc packages", Meta: output.Timestamp(time.Now())}
	sc.Grid(output.Grid{
		NoHeader: true,
		Columns:  []output.Column{{}, {Flex: true}},
		Rows: [][]string{
			{style.Faint("installed"), fmt.Sprintf("%d", stats.TotalPackages)},
			{style.Faint("explicit"), fmt.Sprintf("%d", stats.ExplicitlyInstalled)},
			{style.Faint("foreign / aur"), fmt.Sprintf("%d", stats.ForeignPackages)},
			{style.Faint(fmt.Sprintf("installed in %dd", opts.Days)), fmt.Sprintf("%d", stats.RecentlyInstalled)},
			{style.Faint("installed size"), fmt.Sprintf("%.2f GiB", stats.TotalInstalledSize)},
			{style.Faint("cache size"), stats.CacheSize},
			{style.Faint("orphaned"), fmt.Sprintf("%d", len(stats.OrphanedPackages))},
		},
	})
	if len(stats.OrphanedPackages) > 0 {
		sc.Blank()
		sc.Line(style.Faint("orphaned"))
		for _, pkg := range stats.OrphanedPackages {
			sc.Line(pkg)
		}
	}
	if len(largest) > 0 {
		grid := output.Grid{Columns: []output.Column{
			{Header: "size", Align: output.AlignRight},
			{Header: "package"},
		}}
		for _, pkg := range largest {
			grid.Rows = append(grid.Rows, []string{fmt.Sprintf("%s %s", pkg.Size, pkg.Unit), pkg.Name})
		}
		sc.Blank()
		sc.Line(style.Faint(fmt.Sprintf("largest %d", len(largest))))
		sc.Grid(grid)
	}
	sc.Flush(style.Faint(strings.Join([]string{
		output.Count(stats.TotalPackages, "package", "packages"),
		fmt.Sprintf("%.2f GiB installed", stats.TotalInstalledSize),
		output.Count(len(stats.OrphanedPackages), "orphan", "orphans"),
	}, style.Sep())))
	return nil
}
