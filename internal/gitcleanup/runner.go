package gitcleanup

import "github.com/jyablonski/arc/internal/boundary"

// run is this package's subprocess seam. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock so the git
// invocations can be controlled and asserted without shelling out or touching
// a real repository.
var run boundary.ShellRunner = boundary.DefaultShell
