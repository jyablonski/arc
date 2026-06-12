package hardware

import "github.com/jyablonski/arc/internal/boundary"

// run is this package's subprocess seam. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock so the
// dmidecode/lshw/lspci invocations (several of which need sudo) can be
// controlled and asserted without shelling out or prompting for a password.
var run boundary.ShellRunner = boundary.DefaultShell
