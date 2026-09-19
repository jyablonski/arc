package sysupdate

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/stretchr/testify/require"
)

// TestRunWithDeps_fullRunContract pins the whole screen: one label column,
// one right edge, a single arc-worded gate per section, pacman's gate answered
// by the proxy because it matches the approval, and routine warnings tallied.
func TestRunWithDeps_fullRunContract(t *testing.T) {
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.Stdin = stdinWith(t, "y\ny\n")
	clock := time.Date(2026, 9, 19, 14, 3, 41, 0, time.Local)
	deps.Now = func() time.Time { clock = clock.Add(700 * time.Millisecond); return clock }
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{
			{Name: "vmaf", FromVersion: "3.2.0-1", ToVersion: "3.2.1-1", SizeBytes: 400 * 1024},
			{Name: "xxhash", FromVersion: "0.8.3-1", ToVersion: "0.8.4-1", SizeBytes: 201 * 1024},
		}, nil
	}
	calls := 0
	deps.InstalledVersions = func() (map[string]string, error) {
		calls++
		if calls >= 3 {
			return map[string]string{"archlinux-keyring": "20260909-1", "vmaf": "3.2.1-1", "xxhash": "0.8.4-1"}, nil
		}
		return map[string]string{"archlinux-keyring": "20260909-1", "vmaf": "3.2.0-1", "xxhash": "0.8.3-1"}, nil
	}
	deps.CheckYayAvailable = func() bool { return true }
	fp := 0
	deps.ForeignPackages = func() (map[string]string, error) {
		fp++
		if fp > 1 {
			return map[string]string{"cursor-bin": "3.21.16-1", "google-cloud-cli": "585.0.0-1", "spotify": "1:1.2.96.518-2"}, nil
		}
		return map[string]string{"cursor-bin": "3.21.13-1", "google-cloud-cli": "584.0.0-1", "spotify": "1:1.2.96.518-2"}, nil
	}
	deps.IgnoredPackages = func() ([]string, error) { return []string{"spotify"}, nil }
	now := clock
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return &aurreview.Result{
			Updates: []aurreview.Update{
				{Name: "cursor-bin", PackageBase: "cursor-bin", InstalledVersion: "3.21.13-1", TargetVersion: "3.21.16-1", LastModified: now.Add(-3 * time.Hour).Unix()},
				{Name: "google-cloud-cli", PackageBase: "google-cloud-cli", InstalledVersion: "584.0.0-1", TargetVersion: "585.0.0-1", LastModified: now.Add(-4 * time.Hour).Unix()},
			},
			Changes: map[string]aurreview.Change{
				"cursor-bin":       {Summary: "pkgver + checksums", Routine: true, Baseline: true},
				"google-cloud-cli": {Summary: "pkgver + checksums", Routine: true, Baseline: true},
			},
		}, nil
	}
	deps.CommitAUR = func(*aurreview.Result) error { return nil }
	deps.RunAUR = func(w io.Writer, stdin *os.File, name string, args ...string) error {
		in := bufio.NewReader(stdin)
		_, _ = fmt.Fprint(w, "==> Making package: cursor-bin 3.21.16-1\n==> WARNING: Skipping verification of source file PGP signatures.\n")
		clock = clock.Add(38 * time.Second)
		_, _ = fmt.Fprint(w, "==> Making package: google-cloud-cli 585.0.0-1\n==> WARNING: Skipping verification of source file PGP signatures.\n==> WARNING: Backup entry file not in package : etc/profile.d/x\n")
		clock = clock.Add(12 * time.Second)
		_, _ = fmt.Fprint(w, "Packages (2) cursor-bin-3.21.16-1  google-cloud-cli-585.0.0-1\n\nTotal Installed Size:  691.51 MiB\nNet Upgrade Size:        2.87 MiB\n\n:: Proceed with installation? [Y/n] ")
		ans, _ := in.ReadString('\n')
		require.Equal(t, "y\n", ans)
		_, _ = fmt.Fprint(w, "\n:: Processing package changes...\n")
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{}))
	require.Equal(t, ""+
		"arc update system                                        2026-09-19 14:03:41\n"+
		"────────────────────────────────────────────────────────────────────────────\n\n"+
		"SYNC\n"+
		"  ✓ databases           synchronized                                    0.7s\n"+
		"  · archlinux-keyring   20260909-1  current\n\n"+
		"REPO                                                   2 updates · 601.0 KiB\n"+
		"    vmaf                3.2.0-1 → 3.2.1-1\n"+
		"    xxhash              0.8.3-1 → 0.8.4-1\n\n"+
		"  Upgrade 2 repo packages, 601.0 KiB? [Y/n] \n"+
		"  ✓ vmaf                3.2.1-1\n"+
		"  ✓ xxhash              0.8.4-1\n"+
		"  ✓ hooks               post-transaction complete                       0.7s\n\n"+
		"AUR                                                    2 updates · 1 ignored\n"+
		"    cursor-bin          3.21.13-1 → 3.21.16-1   3h ago  pkgver + checksums\n"+
		"    google-cloud-cli    584.0.0-1 → 585.0.0-1   4h ago  pkgver + checksums\n"+
		"  · spotify             held at 1:1.2.96.518-2          IgnorePkg\n\n"+
		"  ✓ review              no build logic changed                     d to diff\n"+
		"  Upgrade 2 AUR packages? [Y/n] \n"+
		"  ✓ cursor-bin          3.21.16-1                                      38.7s\n"+
		"  ✓ google-cloud-cli    585.0.0-1                                      12.7s\n"+
		"  · disk                2.9 MiB net\n"+
		"  ✓ cache               old archives cleaned                            0.7s\n\n"+
		"────────────────────────────────────────────────────────────────────────────\n"+
		"4 upgraded · 1 ignored · 3 warnings (arc update system -v) · 58.4s\n", out.String())
	for _, line := range strings.Split(out.String(), "\n") {
		require.LessOrEqual(t, len([]rune(line)), defaultRenderWidth, line)
	}
}
