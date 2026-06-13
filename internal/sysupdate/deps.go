package sysupdate

import (
	"context"
	"os"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
)

// Deps is injectable state for [RunWithDeps]; unset fields use [DefaultDeps].
type Deps struct {
	CheckPacman       func() error
	KernelVersions    func() (map[string]string, error)
	RunInteractive    func(name string, args ...string) error
	RunSudo           func(name string, args ...string) (string, error)
	CheckYayAvailable func() bool
	Stdin             *os.File

	// AUR triage seams; nil disables the pre-yay review (e.g. no state path).
	ForeignPackages func() (map[string]string, error)
	IgnoredPackages func() ([]string, error)
	ReviewAUR       func(ctx context.Context, installed map[string]string) (*aurreview.Result, error)
	CommitAUR       func(res *aurreview.Result) error
}

// DefaultDeps returns production dependencies.
func DefaultDeps() Deps {
	d := Deps{
		CheckPacman:       pacman.CheckPacmanAvailable,
		KernelVersions:    pacman.InstalledKernelVersions,
		RunInteractive:    shell.RunInteractive,
		RunSudo:           shell.RunSudo,
		CheckYayAvailable: pacman.CheckYayAvailable,
		Stdin:             os.Stdin,
		ForeignPackages:   pacman.GetForeignPackageVersions,
		IgnoredPackages:   pacman.GetIgnoredPackages,
	}
	if path, err := aurreview.DefaultStatePath(); err == nil {
		rv := aurreview.New(path)
		d.ReviewAUR = rv.Review
		d.CommitAUR = rv.Commit
	}
	return d
}

func mergeDeps(override Deps) Deps {
	d := DefaultDeps()
	if override.CheckPacman != nil {
		d.CheckPacman = override.CheckPacman
	}
	if override.KernelVersions != nil {
		d.KernelVersions = override.KernelVersions
	}
	if override.RunInteractive != nil {
		d.RunInteractive = override.RunInteractive
	}
	if override.RunSudo != nil {
		d.RunSudo = override.RunSudo
	}
	if override.CheckYayAvailable != nil {
		d.CheckYayAvailable = override.CheckYayAvailable
	}
	if override.Stdin != nil {
		d.Stdin = override.Stdin
	}
	if override.ForeignPackages != nil {
		d.ForeignPackages = override.ForeignPackages
	}
	if override.IgnoredPackages != nil {
		d.IgnoredPackages = override.IgnoredPackages
	}
	if override.ReviewAUR != nil {
		d.ReviewAUR = override.ReviewAUR
	}
	if override.CommitAUR != nil {
		d.CommitAUR = override.CommitAUR
	}
	return d
}
