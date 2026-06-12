package pacman

import (
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
)

// setRunner swaps this package's subprocess seam for the duration of a test and
// restores it afterwards via t.Cleanup, so swaps never leak between tests.
func setRunner(t *testing.T, m boundary.ShellRunner) {
	t.Helper()
	prev := run
	run = m
	t.Cleanup(func() { run = prev })
}
