package sysupdate

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderer_happyPathContract(t *testing.T) {
	var out bytes.Buffer
	r := NewRenderer(&out, false)
	started := time.Date(2026, 8, 13, 7, 51, 25, 0, time.Local)

	r.RunHeader(started)
	r.Section("SYNC", "")
	r.InfoResult("archlinux-keyring", "20260727-1  current")
	r.Result("databases", "synchronized", 1400*time.Millisecond)
	r.Blank()
	changes := []PackageChange{
		{Name: "procps-ng", FromVersion: "4.0.6-1", ToVersion: "4.0.7-1", SizeBytes: 1024 * 1024},
		{Name: "mbedtls3", ToVersion: "3.6.7-1", Note: "new dep", SizeBytes: 2 * 1024 * 1024},
		{Name: "bolt", FromVersion: "0.9.11-2", ToVersion: "0.9.11-3", SizeBytes: 512 * 1024},
	}
	r.Section("REPO", "3 updates · 3.5 MiB")
	r.Plan(changes)
	r.Blank()
	r.Prompt("Upgrade 3 repo packages, 3.5 MiB?", true)
	_, _ = out.WriteString("y\n")
	for _, change := range changes {
		r.PackageResult(change, 0)
	}
	r.Result("hooks", "post-transaction complete", 2100*time.Millisecond)
	r.Footer(64*time.Second, "")

	require.Equal(t, ""+
		"arc update system                                        2026-08-13 07:51:25\n"+
		"────────────────────────────────────────────────────────────────────────────\n\n"+
		"SYNC\n"+
		"  · archlinux-keyring   20260727-1  current\n"+
		"  ✓ databases           synchronized                                    1.4s\n\n"+
		"REPO                                                     3 updates · 3.5 MiB\n"+
		"    bolt                0.9.11-2 → 0.9.11-3\n"+
		"    mbedtls3            —        → 3.6.7-1   new dep\n"+
		"    procps-ng           4.0.6-1  → 4.0.7-1\n\n"+
		"  Upgrade 3 repo packages, 3.5 MiB? [Y/n] y\n"+
		"  ✓ procps-ng           4.0.7-1\n"+
		"  ✓ mbedtls3            3.6.7-1  installed\n"+
		"  ✓ bolt                0.9.11-3\n"+
		"  ✓ hooks               post-transaction complete                       2.1s\n"+
		"\n────────────────────────────────────────────────────────────────────────────\n"+
		"0 upgraded · 1m 04s\n", out.String())
}

func TestRenderer_failureTailContract(t *testing.T) {
	var out bytes.Buffer
	r := Renderer{Out: &out}
	r.Error("pacman update failed")
	r.FailureTail([]string{"error: failed to commit transaction", "Errors occurred, no packages were upgraded."})
	r.LogPath("/tmp/arc-update.log")

	require.Equal(t, ""+
		"  ✗ pacman update failed\n"+
		"\n"+
		"  subprocess output:\n"+
		"    error: failed to commit transaction\n"+
		"    Errors occurred, no packages were upgraded.\n"+
		"  · log                 /tmp/arc-update.log\n", out.String())
}

func TestRenderer_AURAndIgnoredContract(t *testing.T) {
	var out bytes.Buffer
	r := Renderer{Out: &out, Width: 60}
	r.Section("AUR", "1 update · 1 ignored")
	r.Plan(
		[]PackageChange{{Name: "cursor-bin", FromVersion: "3.15.19-1", ToVersion: "3.16.13-1", Note: "6h ago", Change: "source + build()", Attention: true}},
		ignoredPackage{Name: "spotify", Version: "1.2.3-1"},
	)

	// Held packages share the plan's columns instead of drifting out of them.
	require.Equal(t, ""+
		"AUR                                     1 update · 1 ignored\n"+
		"    cursor-bin          3.15.19-1 → 3.16.13-1  6h ago  source + build()\n"+
		"  · spotify             held at 1.2.3-1                IgnorePkg\n", out.String())
}

func TestRenderer_longNameKeepsPlanColumns(t *testing.T) {
	var out bytes.Buffer
	r := Renderer{Out: &out}
	r.Plan([]PackageChange{
		{Name: "visual-studio-code-bin", FromVersion: "1.0-1", ToVersion: "1.1-1", Change: "pkgver"},
		{Name: "vmaf", FromVersion: "3.2.0-1", ToVersion: "3.2.1-1", Change: "pkgver"},
	})
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Equal(t, strings.Index(lines[0], "→"), strings.Index(lines[1], "→"))
	require.Equal(t, strings.Index(lines[0], "pkgver"), strings.Index(lines[1], "pkgver"))
}
