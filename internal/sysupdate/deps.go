package sysupdate

import (
	"os"

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
}

// DefaultDeps returns production dependencies.
func DefaultDeps() Deps {
	return Deps{
		CheckPacman:       pacman.CheckPacmanAvailable,
		KernelVersions:    pacman.InstalledKernelVersions,
		RunInteractive:    shell.RunInteractive,
		RunSudo:           shell.RunSudo,
		CheckYayAvailable: pacman.CheckYayAvailable,
		Stdin:             os.Stdin,
	}
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
	return d
}
