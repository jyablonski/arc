package setupdeps

import "github.com/jyablonski/arc/internal/boundary"

// run is this package's subprocess seam. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock so the
// availability probes and package installs can be controlled and asserted
// without shelling out or actually installing anything.
var run boundary.ShellRunner = boundary.DefaultShell
