package claude

import "github.com/jyablonski/arc/internal/boundary"

// run is this package's subprocess seam. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock so the macOS
// Keychain lookup can be controlled without shelling out to `security`.
var run boundary.ShellRunner = boundary.DefaultShell
