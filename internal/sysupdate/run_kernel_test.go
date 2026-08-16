package sysupdate

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/stretchr/testify/require"
)

func TestRunWithDeps_kernelBump_promptRebootNo(t *testing.T) {
	f, err := os.CreateTemp("", "sysupdate-stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("y\nn\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	before := map[string]string{"linux": "1:6.6.1-1"}
	after := map[string]string{"linux": "1:6.6.2-1"}
	calls := 0
	deps := testDepsKernelStable(t)
	deps.KernelVersions = func() (map[string]string, error) {
		if calls == 0 {
			calls++
			return before, nil
		}
		return after, nil
	}
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "linux", FromVersion: "1:6.6.1-1", ToVersion: "1:6.6.2-1"}}, nil
	}
	deps.InstalledVersions = func() (map[string]string, error) {
		return map[string]string{"archlinux-keyring": "20260727-1", "linux": "1:6.6.2-1"}, nil
	}
	deps.Stdin = f

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
}

func TestRunWithDeps_kernelBump_rebootDeferredAfterAURAndCache(t *testing.T) {
	f, err := os.CreateTemp("", "sysupdate-stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("y\ny\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	calls := 0
	var order []string
	deps := testDepsKernelStable(t)
	deps.KernelVersions = func() (map[string]string, error) {
		if calls == 0 {
			calls++
			return map[string]string{"linux": "1:6.6.1-1"}, nil
		}
		return map[string]string{"linux": "1:6.6.2-1"}, nil
	}
	deps.RunInteractive = func(name string, args ...string) error {
		order = append(order, name+" "+strings.Join(args, " "))
		return nil
	}
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		order = append(order, name+" "+strings.Join(args, " "))
		return nil
	}
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = testForeignPackageUpgrade()
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	deps.CommitAUR = func(*aurreview.Result) error { return nil }
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "linux", FromVersion: "1:6.6.1-1", ToVersion: "1:6.6.2-1"}}, nil
	}
	deps.InstalledVersions = func() (map[string]string, error) {
		return map[string]string{"archlinux-keyring": "20260727-1", "linux": "1:6.6.2-1"}, nil
	}
	deps.Stdin = f

	require.NoError(t, RunWithDeps(deps, Options{}))

	joined := strings.Join(order, "\n")
	require.Contains(t, joined, "yay -Syu --aur")
	require.Contains(t, joined, "paccache -rv")
	require.Contains(t, joined, "reboot")
	require.Greater(t, indexOf(order, "reboot"), indexOf(order, "yay"))
	require.Greater(t, indexOf(order, "reboot"), indexOf(order, "paccache"))
}

func TestRunWithDeps_kernelVersionsWarnBefore(t *testing.T) {
	f, err := os.CreateTemp("", "sysupdate-kern-warn")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("n\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	calls := 0
	deps := testDepsKernelStable(t)
	deps.Stdin = f
	deps.KernelVersions = func() (map[string]string, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("rpm db locked")
		}
		return map[string]string{"linux": "1:1-1"}, nil
	}
	deps.RunInteractive = func(string, ...string) error { return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
}

func indexOf(calls []string, prefix string) int {
	for i, call := range calls {
		if strings.HasPrefix(call, prefix) || strings.Contains(call, " "+prefix) {
			return i
		}
	}
	return -1
}
