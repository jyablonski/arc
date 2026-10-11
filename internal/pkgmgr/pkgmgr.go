package pkgmgr

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jyablonski/arc/internal/platform"
)

var ErrUnsupportedPlatform = errors.New("package management is not supported on this platform")

type UpdateOptions struct {
	SkipAUR   bool
	SkipCache bool
	AssumeYes bool
	Log       bool
	Verbose   bool
	ShowDiff  bool
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

type InstallOptions struct {
	Packages []string
	AUR      bool
	Cask     bool
	Formula  bool
}

func (opts InstallOptions) validate() error {
	if len(opts.Packages) == 0 {
		return fmt.Errorf("at least one package is required")
	}
	for _, name := range opts.Packages {
		if name == "" || strings.HasPrefix(name, "-") || strings.ContainsFunc(name, unicode.IsSpace) {
			return fmt.Errorf("invalid package name %q", name)
		}
	}
	selected := 0
	for _, enabled := range []bool{opts.AUR, opts.Cask, opts.Formula} {
		if enabled {
			selected++
		}
	}
	if selected > 1 {
		return fmt.Errorf("only one of --aur, --cask, or --formula may be used")
	}
	return nil
}

//go:generate go tool moq -rm -out manager_moq.go . Manager
type Manager interface {
	UpdateSystem(opts UpdateOptions) error
	Install(opts InstallOptions) error
	Clean(opts CleanOptions) error
	Installed(opts InstalledOptions) error
	Packages(opts PackageOptions) error
}

func New(os platform.OS) Manager {
	switch os {
	case platform.Linux:
		return linuxManager{pac: realPacman{}}
	case platform.Darwin:
		return darwinManager{}
	default:
		return unsupportedManager{}
	}
}
