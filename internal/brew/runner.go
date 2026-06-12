package brew

import "github.com/jyablonski/arc/internal/boundary"

// run is this package's subprocess seam. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock so the
// brew/du invocations can be controlled and asserted without shelling out.
var run boundary.ShellRunner = boundary.DefaultShell
