package pkgmgr

import (
	"errors"

	"github.com/jyablonski/arc/internal/platform"
)

var ErrUnsupportedPlatform = errors.New("package management is not supported on this platform")

type UpdateOptions struct {
	SkipAUR   bool
	SkipCache bool
}

type CleanOptions struct {
	OrphansOnly bool
	CacheOnly   bool
}

type InstalledOptions struct {
	ForeignOnly bool
	Count       bool
}

type PackageOptions struct {
	Days int
	Top  int
	JSON bool
}

type Manager interface {
	UpdateSystem(opts UpdateOptions) error
	Clean(opts CleanOptions) error
	Installed(opts InstalledOptions) error
	Packages(opts PackageOptions) error
}

func New(os platform.OS) Manager {
	switch os {
	case platform.Linux:
		return linuxManager{}
	case platform.Darwin:
		return darwinManager{}
	default:
		return unsupportedManager{}
	}
}
