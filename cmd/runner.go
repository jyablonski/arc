package cmd

import "github.com/jyablonski/arc/internal/boundary"

// run is the cmd layer's subprocess seam for the handful of commands that shell
// out directly (docker/info/update-uv/validate/aws guards) rather than
// delegating to an internal package. Production uses the real shell via
// boundary.DefaultShell; tests swap in a boundary.ShellRunnerMock.
var run boundary.ShellRunner = boundary.DefaultShell
