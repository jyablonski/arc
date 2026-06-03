package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os"

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
	if !shell.CommandExists("brew") {
		return shell.NewErrToolNotAvailable("brew")
	}

	output.Info("Updating Homebrew...")
	if err := shell.RunInteractive("brew", "update"); err != nil {
		return fmt.Errorf("brew update failed: %w", err)
	}

	output.Info("Upgrading Homebrew packages...")
	if err := shell.RunInteractive("brew", "upgrade"); err != nil {
		return fmt.Errorf("brew upgrade failed: %w", err)
	}

	if !opts.SkipCache {
		output.Info("Cleaning Homebrew cache...")
		if err := shell.RunInteractive("brew", "cleanup"); err != nil {
			return fmt.Errorf("brew cleanup failed: %w", err)
		}
	}

	output.Success("Homebrew update complete")
	return nil
}

func (darwinManager) Clean(opts CleanOptions) error {
	if err := brew.CheckAvailable(); err != nil {
		return err
	}

	if !opts.OrphansOnly {
		output.Header("cleaning Homebrew cache")
		if err := shell.RunInteractive("brew", "cleanup"); err != nil {
			return fmt.Errorf("failed to clean Homebrew cache: %w", err)
		}
		output.Success("Homebrew cache cleaned")
	}

	if !opts.CacheOnly {
		output.Header("removing unused Homebrew dependencies")
		if err := shell.RunInteractive("brew", "autoremove"); err != nil {
			return fmt.Errorf("failed to autoremove Homebrew dependencies: %w", err)
		}
		output.Success("Homebrew autoremove complete")
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

	output.Header("=== Homebrew Formulae ===")
	fmt.Printf("%d\n\n", stats.Formulae)
	output.Header("=== Homebrew Casks ===")
	fmt.Printf("%d\n\n", stats.Casks)
	output.Header("=== Homebrew Leaves ===")
	fmt.Printf("%d\n\n", stats.Leaves)
	output.Header("=== Homebrew Cache Size ===")
	if stats.CacheSize == "" {
		fmt.Println("Unavailable")
	} else {
		fmt.Println(stats.CacheSize)
	}
	fmt.Println()
	output.Header(fmt.Sprintf("=== Leaf Formulae (%d) ===", len(leaves)))
	if len(leaves) == 0 {
		fmt.Println("None")
		return nil
	}
	for _, leaf := range leaves {
		fmt.Println(leaf)
	}
	return nil
}
