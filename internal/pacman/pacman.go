package pacman

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/shell"
)

// ErrParseCacheSize is returned when the package cache size output cannot be parsed.
var ErrParseCacheSize = errors.New("could not parse cache size")

// PackageInfo represents package information
type PackageInfo struct {
	Name          string
	Size          string
	Unit          string
	InstalledSize float64 // in MiB
}

// GetPackageCount returns the total number of installed packages
func GetPackageCount() (int, error) {
	output, err := shell.Run("pacman", "-Q")
	if err != nil {
		return 0, err
	}
	if output == "" {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(output), "\n")), nil
}

// GetExplicitlyInstalledCount returns the count of explicitly installed packages
func GetExplicitlyInstalledCount() (int, error) {
	output, err := shell.Run("pacman", "-Qe")
	if err != nil {
		return 0, err
	}
	if output == "" {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(output), "\n")), nil
}

// GetForeignPackageCount returns the count of foreign/AUR packages
func GetForeignPackageCount() (int, error) {
	output, err := shell.Run("pacman", "-Qm")
	if err != nil {
		return 0, err
	}
	if output == "" {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(output), "\n")), nil
}

// GetExplicitlyInstalled returns a list of explicitly installed packages
func GetExplicitlyInstalled() ([]string, error) {
	output, err := shell.Run("pacman", "-Qe")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// GetForeignPackages returns a list of foreign/AUR packages
func GetForeignPackages() ([]string, error) {
	output, err := shell.Run("pacman", "-Qm")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// GetTotalInstalledSize returns the total installed size in GiB
func GetTotalInstalledSize() (float64, error) {
	output, err := shell.Run("pacman", "-Qi")
	if err != nil {
		return 0, err
	}

	var totalMiB float64
	lines := strings.Split(output, "\n")
	re := regexp.MustCompile(`Installed Size\s+:\s+([\d.]+)\s+(KiB|MiB|GiB)`)

	for _, line := range lines {
		if strings.HasPrefix(line, "Installed Size") {
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				size, err := strconv.ParseFloat(matches[1], 64)
				if err != nil {
					continue
				}
				unit := matches[2]

				// Convert to MiB
				switch unit {
				case "KiB":
					totalMiB += size / 1024
				case "MiB":
					totalMiB += size
				case "GiB":
					totalMiB += size * 1024
				}
			}
		}
	}

	// Convert MiB to GiB
	return totalMiB / 1024, nil
}

// GetCacheSize returns the package cache size
func GetCacheSize() (string, error) {
	output, err := shell.Run("du", "-sh", "/var/cache/pacman/pkg")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(output)
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", ErrParseCacheSize
}

// GetOrphanedPackages returns a list of orphaned packages
func GetOrphanedPackages() ([]string, error) {
	output, err := shell.Run("pacman", "-Qdt")
	if err != nil {
		// No orphans is not an error
		if strings.Contains(err.Error(), "exit status 1") {
			return []string{}, nil
		}
		return nil, err
	}
	if output == "" {
		return []string{}, nil
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	packages := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				packages = append(packages, parts[0])
			}
		}
	}
	return packages, nil
}

// GetRecentlyInstalledCount returns the count of packages installed in the last N days
func GetRecentlyInstalledCount(days int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	cutoffStr := cutoff.Format("2006-01-02")

	output, err := shell.Run("grep", "-E", `\[ALPM\] installed`, "/var/log/pacman.log")
	if err != nil {
		return 0, err
	}

	if output == "" {
		return 0, nil
	}

	lines := strings.Split(output, "\n")
	count := 0
	for _, line := range lines {
		if len(line) >= 11 {
			dateStr := line[1:11] // Extract date from [YYYY-MM-DD]
			if dateStr >= cutoffStr {
				count++
			}
		}
	}

	return count, nil
}

// GetLargestPackages returns the N largest packages
func GetLargestPackages(topN int) ([]PackageInfo, error) {
	output, err := shell.Run("pacman", "-Qi")
	if err != nil {
		return nil, err
	}

	packages := make(map[string]PackageInfo)
	lines := strings.Split(output, "\n")
	re := regexp.MustCompile(`Installed Size\s+:\s+([\d.]+)\s+(KiB|MiB|GiB)`)
	var currentName string

	for _, line := range lines {
		if strings.HasPrefix(line, "Name") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				currentName = parts[2]
			}
		} else if strings.HasPrefix(line, "Installed Size") && currentName != "" {
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				size, err := strconv.ParseFloat(matches[1], 64)
				if err != nil {
					continue
				}
				unit := matches[2]

				var sizeMiB float64
				switch unit {
				case "KiB":
					sizeMiB = size / 1024
				case "MiB":
					sizeMiB = size
				case "GiB":
					sizeMiB = size * 1024
				}

				packages[currentName] = PackageInfo{
					Name:          currentName,
					Size:          matches[1],
					Unit:          unit,
					InstalledSize: sizeMiB,
				}
			}
		}
	}

	// Convert to slice and sort by size
	result := make([]PackageInfo, 0, len(packages))
	for _, pkg := range packages {
		result = append(result, pkg)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].InstalledSize > result[j].InstalledSize
	})

	if topN > len(result) {
		topN = len(result)
	}

	return result[:topN], nil
}

// SearchPackages searches for packages
func SearchPackages(query string) ([]string, error) {
	output, err := shell.Run("pacman", "-Ss", query)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// CheckPacmanAvailable checks if pacman is available
func CheckPacmanAvailable() error {
	if !shell.CommandExists("pacman") {
		return shell.NewErrToolNotAvailable("pacman")
	}
	return nil
}

// CheckYayAvailable checks if yay is available
func CheckYayAvailable() bool {
	return shell.CommandExists("yay")
}
