package pkgmgr

import "github.com/jyablonski/arc/internal/pacman"

// pacmanOps is the subset of the pacman package that linuxManager depends on.
// It's a collaborator seam: production wires realPacman (the actual pacman
// package), while tests inject a pacmanOpsMock so linuxManager's Clean/Installed/
// Packages orchestration can be exercised deterministically without a real
// pacman on the host.
//
// This mock mixes stdlib (sync) with a third-party import (pacman); moq's
// default gofmt leaves them in one group, which fails the goimports CI check,
// so -fmt goimports splits and orders the import groups at generation time.
//
//go:generate go tool moq -rm -fmt goimports -out pacmanops_moq.go . pacmanOps
type pacmanOps interface {
	CheckPacmanAvailable() error
	GetOrphanedPackages() ([]string, error)
	GetForeignPackages() ([]string, error)
	GetExplicitlyInstalled() ([]string, error)
	GetPackageCount() (int, error)
	GetExplicitlyInstalledCount() (int, error)
	GetForeignPackageCount() (int, error)
	GetTotalInstalledSize() (float64, error)
	GetCacheSize() (string, error)
	GetRecentlyInstalledCount(days int) (int, error)
	GetLargestPackages(topN int) ([]pacman.PackageInfo, error)
}

// realPacman forwards to the real pacman package.
type realPacman struct{}

func (realPacman) CheckPacmanAvailable() error            { return pacman.CheckPacmanAvailable() }
func (realPacman) GetOrphanedPackages() ([]string, error) { return pacman.GetOrphanedPackages() }
func (realPacman) GetForeignPackages() ([]string, error)  { return pacman.GetForeignPackages() }
func (realPacman) GetExplicitlyInstalled() ([]string, error) {
	return pacman.GetExplicitlyInstalled()
}
func (realPacman) GetPackageCount() (int, error) { return pacman.GetPackageCount() }
func (realPacman) GetExplicitlyInstalledCount() (int, error) {
	return pacman.GetExplicitlyInstalledCount()
}
func (realPacman) GetForeignPackageCount() (int, error) { return pacman.GetForeignPackageCount() }
func (realPacman) GetTotalInstalledSize() (float64, error) {
	return pacman.GetTotalInstalledSize()
}
func (realPacman) GetCacheSize() (string, error) { return pacman.GetCacheSize() }
func (realPacman) GetRecentlyInstalledCount(days int) (int, error) {
	return pacman.GetRecentlyInstalledCount(days)
}
func (realPacman) GetLargestPackages(topN int) ([]pacman.PackageInfo, error) {
	return pacman.GetLargestPackages(topN)
}
