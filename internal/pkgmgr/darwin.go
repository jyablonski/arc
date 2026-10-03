package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/brew"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
)

type darwinManager struct{}

type brewPackageStats struct {
	Platform          string   `json:"platform"`
	Formulae          int      `json:"formulae"`
	Casks             int      `json:"casks"`
	Leaves            int      `json:"leaves"`
	CacheSize         string   `json:"cache_size,omitempty"`
	LeafFormulae      []string `json:"leaf_formulae,omitempty"`
	InstalledCasks    []string `json:"installed_casks,omitempty"`
	InstalledFormulae []string `json:"installed_formulae,omitempty"`
}

func (darwinManager) UpdateSystem(opts UpdateOptions) error {
	if !run.CommandExists("brew") {
		return shell.NewErrToolNotAvailable("brew")
	}

	started := time.Now()
	output.Title("arc update system", output.Timestamp(started))
	output.Section("homebrew update")
	if err := run.RunInteractive("brew", "update"); err != nil {
		return fmt.Errorf("brew update failed: %w", err)
	}

	output.Section("homebrew upgrade")
	if err := run.RunInteractive("brew", "upgrade"); err != nil {
		return fmt.Errorf("brew upgrade failed: %w", err)
	}

	if !opts.SkipCache {
		output.Section("homebrew cleanup")
		if err := run.RunInteractive("brew", "cleanup"); err != nil {
			return fmt.Errorf("brew cleanup failed: %w", err)
		}
	}

	output.Summary(output.GlyphOK, "homebrew update complete", output.Duration(time.Since(started)))
	return nil
}

func (darwinManager) Clean(opts CleanOptions) error {
	if err := brew.CheckAvailable(); err != nil {
		return err
	}

	if !opts.OrphansOnly {
		output.Section("homebrew cache")
		if err := run.RunInteractive("brew", "cleanup"); err != nil {
			return fmt.Errorf("failed to clean Homebrew cache: %w", err)
		}
		output.Success("homebrew cache cleaned")
	}

	if !opts.CacheOnly {
		output.Section("unused dependencies")
		if err := run.RunInteractive("brew", "autoremove"); err != nil {
			return fmt.Errorf("failed to autoremove Homebrew dependencies: %w", err)
		}
		output.Success("unused dependencies removed")
	}

	return nil
}

func (darwinManager) Installed(opts InstalledOptions) error {
	if opts.ForeignOnly {
		return arcerrs.ErrAUROnlyLinuxOnly
	}
	if err := brew.CheckAvailable(); err != nil {
		return err
	}

	packages, err := brew.ListFormulae()
	if err != nil {
		return fmt.Errorf("failed to get Homebrew formulae: %w", err)
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

func (darwinManager) Packages(opts PackageOptions) error {
	if err := brew.CheckAvailable(); err != nil {
		return err
	}

	formulae, err := brew.ListFormulae()
	if err != nil {
		return fmt.Errorf("failed to list Homebrew formulae: %w", err)
	}
	casks, err := brew.ListCasks()
	if err != nil {
		return fmt.Errorf("failed to list Homebrew casks: %w", err)
	}
	leaves, err := brew.Leaves()
	if err != nil {
		return fmt.Errorf("failed to list Homebrew leaves: %w", err)
	}
	cacheSize, err := brew.CacheSize()
	if err != nil {
		cacheSize = ""
	}

	stats := brewPackageStats{
		Platform:          platform.Darwin.String(),
		Formulae:          len(formulae),
		Casks:             len(casks),
		Leaves:            len(leaves),
		CacheSize:         cacheSize,
		LeafFormulae:      leaves,
		InstalledCasks:    casks,
		InstalledFormulae: formulae,
	}

	if opts.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)
	}

	style := output.StyleFor(os.Stdout)
	sc := &output.Screen{W: os.Stdout, Style: style, Title: "arc packages", Meta: output.Timestamp(time.Now())}
	cache := stats.CacheSize
	if cache == "" {
		cache = style.Dash()
	}
	sc.Grid(output.Grid{
		NoHeader: true,
		Columns:  []output.Column{{}, {Flex: true}},
		Rows: [][]string{
			{style.Faint("formulae"), fmt.Sprintf("%d", stats.Formulae)},
			{style.Faint("casks"), fmt.Sprintf("%d", stats.Casks)},
			{style.Faint("leaves"), fmt.Sprintf("%d", stats.Leaves)},
			{style.Faint("cache size"), cache},
		},
	})
	if len(leaves) > 0 {
		sc.Blank()
		sc.Line(style.Faint("leaf formulae"))
		for _, leaf := range leaves {
			sc.Line(leaf)
		}
	}
	sc.Flush(style.Faint(strings.Join([]string{
		output.Count(stats.Formulae, "formula", "formulae"),
		output.Count(stats.Casks, "cask", "casks"),
		output.Count(stats.Leaves, "leaf", "leaves"),
	}, style.Sep())))
	return nil
}
