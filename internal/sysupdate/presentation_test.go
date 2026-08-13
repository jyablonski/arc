package sysupdate

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderer_happyPathContract(t *testing.T) {
	var out bytes.Buffer
	r := Renderer{Out: &out, Width: 76}
	started := time.Date(2026, 8, 13, 7, 51, 25, 0, time.Local)

	r.RunHeader(started)
	r.Section("SYNC", "")
	r.Result("archlinux-keyring", "20260727-1 (current)", 0)
	r.Result("databases", "core, extra, multilib", 1400*time.Millisecond)
	r.Blank()
	changes := []PackageChange{
		{Name: "procps-ng", FromVersion: "4.0.6-1", ToVersion: "4.0.7-1", SizeBytes: 1024 * 1024},
		{Name: "mbedtls3", ToVersion: "3.6.7-1", Note: "new dep", SizeBytes: 2 * 1024 * 1024},
		{Name: "bolt", FromVersion: "0.9.11-2", ToVersion: "0.9.11-3", SizeBytes: 512 * 1024},
	}
	r.Section("REPO", "3 updates")
	r.Plan(changes)
	r.Prompt("Proceed with 3 repo upgrades?")
	_, _ = out.WriteString("y\n\n")
	r.Result("downloaded", "3 packages, 3.5 MiB", 400*time.Millisecond)
	r.Result("verified", "keys, integrity, conflicts, space", 0)
	for _, change := range changes {
		r.PackageResult(change)
	}
	r.Result("hooks", "3 post-transaction", 2100*time.Millisecond)

	require.Equal(t, ""+
		"arc update system                                        2026-08-13 07:51:25\n"+
		"────────────────────────────────────────────────────────────────────────────\n\n"+
		"SYNC\n"+
		"  ✓ archlinux-keyring        20260727-1 (current)\n"+
		"  ✓ databases                core, extra, multilib                      1.4s\n\n"+
		"REPO                                                               3 updates\n"+
		"  bolt       0.9.11-2 → 0.9.11-3\n"+
		"  mbedtls3   —        → 3.6.7-1   new dep\n"+
		"  procps-ng  4.0.6-1  → 4.0.7-1\n\n"+
		"  download 3.5 MiB\n\n"+
		"  Proceed with 3 repo upgrades? [Y/n] y\n\n"+
		"  ✓ downloaded               3 packages, 3.5 MiB                        0.4s\n"+
		"  ✓ verified                 keys, integrity, conflicts, space\n"+
		"  ✓ procps-ng                4.0.7-1\n"+
		"  ✓ mbedtls3                 3.6.7-1  installed\n"+
		"  ✓ bolt                     0.9.11-3\n"+
		"  ✓ hooks                    3 post-transaction                         2.1s\n", out.String())
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
		"\n"+
		"  log /tmp/arc-update.log\n", out.String())
}

func TestRenderer_AURAndIgnoredContract(t *testing.T) {
	var out bytes.Buffer
	r := Renderer{Out: &out, Width: 60}
	r.Section("AUR", "1 update · 1 ignored")
	r.Plan([]PackageChange{{Name: "cursor-bin", FromVersion: "3.15.19-1", ToVersion: "3.16.13-1", Note: "published 6h ago"}})
	r.Warning("spotify 1.2.3-1 ignored by IgnorePkg")

	require.Equal(t, ""+
		"AUR                                     1 update · 1 ignored\n"+
		"  cursor-bin  3.15.19-1 → 3.16.13-1  published 6h ago\n"+
		"  ⚠ spotify 1.2.3-1 ignored by IgnorePkg\n", out.String())
}

func TestFormatDuration_subTenth(t *testing.T) {
	require.Equal(t, "<0.1s", formatDuration(20*time.Millisecond))
}
