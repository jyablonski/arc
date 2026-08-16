package sysupdate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
)

const pacmanPlanFormat = "%n|%v|%s|%R"

func resolveRepoPlan() ([]PackageChange, error) {
	installed, err := pacman.GetInstalledPackageVersions()
	if err != nil {
		return nil, fmt.Errorf("read installed package versions: %w", err)
	}
	out, err := shell.Run("pacman", "-Sup", "--print-format", pacmanPlanFormat)
	if err != nil {
		return nil, fmt.Errorf("resolve pacman transaction: %w", err)
	}
	return parsePacmanPlan(out, installed)
}

func parsePacmanPlan(output string, installed map[string]string) ([]PackageChange, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}
	var changes []PackageChange
	seen := map[string]bool{}
	for lineNo, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 4 || fields[0] == "" || fields[1] == "" {
			return nil, fmt.Errorf("parse pacman plan line %d: expected name|version|size|replaces, got %q", lineNo+1, line)
		}
		if seen[fields[0]] {
			return nil, fmt.Errorf("parse pacman plan line %d: duplicate package %q", lineNo+1, fields[0])
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("parse pacman plan line %d: invalid size %q", lineNo+1, fields[2])
		}
		change := PackageChange{
			Name:        fields[0],
			FromVersion: installed[fields[0]],
			ToVersion:   fields[1],
			SizeBytes:   size,
		}
		change.Replaces = activeReplacements(fields[3], installed)
		if change.FromVersion == "" {
			if change.Replaces != "" {
				change.Note = "replaces " + strings.ReplaceAll(change.Replaces, ",", ", ")
			} else {
				change.Note = "new dep"
			}
		}
		changes = append(changes, change)
		seen[change.Name] = true
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return changes, nil
}

func activeReplacements(raw string, installed map[string]string) string {
	var active []string
	for _, replacement := range strings.Fields(raw) {
		name := replacement
		if i := strings.IndexAny(name, "<>="); i >= 0 {
			name = name[:i]
		}
		if installed[name] != "" {
			active = append(active, name)
		}
	}
	sort.Strings(active)
	return strings.Join(active, ",")
}

func samePackagePlan(a, b []PackageChange) bool {
	a = normalizedPlan(a)
	b = normalizedPlan(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
