package pkgmgr

type unsupportedManager struct{}

func (unsupportedManager) UpdateSystem(UpdateOptions) error { return ErrUnsupportedPlatform }
func (unsupportedManager) Clean(CleanOptions) error         { return ErrUnsupportedPlatform }
func (unsupportedManager) Installed(InstalledOptions) error { return ErrUnsupportedPlatform }
func (unsupportedManager) Packages(PackageOptions) error    { return ErrUnsupportedPlatform }
