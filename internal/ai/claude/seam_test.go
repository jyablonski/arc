package claude

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

// setGOOS overrides the OS gate used by the Keychain fallback for the duration
// of a test, so the darwin branch is exercisable on any host.
func setGOOS(t *testing.T, os string) {
	t.Helper()
	prev := goos
	goos = os
	t.Cleanup(func() { goos = prev })
}
