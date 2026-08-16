package sysupdate

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAUROutput_reducesCapturedYayChatter(t *testing.T) {
	segments := []string{
		":: 5 dependencies will also be installed for this operation.\n" +
			"   extra/python-build -> 1.4.3-1\n" +
			"   aur/python-gevent-eventemitter -> 2.1-6\n\n" +
			"1 aur/python-steam 2.0.0.alpha1-2 -> 2.0.0.alpha1-3\n" +
			"==> Packages to exclude:\n" +
			"==> [N]one [A]ll [Ab]ort or (1 2 3)\n" +
			"==> ",
		":: (1/2) Downloaded PKGBUILD: python-steam\n" +
			"2 python-steam (Installed)\n" +
			"1 python-gevent-eventemitter\n" +
			"==> Diffs to show?\n" +
			"==> [N]one [A]ll [Ab]ort or (1 2 3)\n" +
			"==> ",
		"diff --git a/PKGBUILD b/PKGBUILD\n-pkgrel=2\n+pkgrel=3\n" +
			"2 python-steam (Installed)\n" +
			"1 python-gevent-eventemitter\n" +
			"==> PKGBUILDs to edit?\n" +
			"==> [N]one [A]ll [Ab]ort or (1 2 3)\n" +
			"==> ",
		"==> Making package: python-steam 2.0.0.alpha1-3\n" +
			"==> Retrieving sources...\n" +
			"  % Total % Received % Xferd Average Speed\n" +
			"100 659.5k 0 659.5k 0 0 1.09M\n" +
			"==> WARNING: Skipping verification of source file PGP signatures.\n" +
			"==> WARNING: Skipping verification of source file PGP signatures.\n" +
			"==> Validating source files with sha256sums...\n" +
			"steam.tar.gz ... Passed\n" +
			"running egg_info\n" +
			"copying hundreds/of/files.py\n" +
			"==> Starting check()...\n" +
			"================ 55 passed in 0.53s ================\n" +
			":: Proceed with installation? [Y/n] ",
		"Packages (2) python-steam python-gevent-eventemitter\n" +
			"Total Download Size: 3.40 MiB\n" +
			"Total Installed Size: 21.09 MiB\n\n" +
			":: Retrieving packages...\n" +
			"python-steam downloading...\n" +
			"checking package integrity...\n" +
			":: Processing package changes...\n" +
			"upgrading python-steam...\n",
	}
	raw := []byte(strings.Join(segments, ""))

	var log bytes.Buffer
	var terminal bytes.Buffer
	reduced := newAUROutput(&log, Renderer{Out: &terminal})
	for _, segment := range segments {
		writeInChunks(t, reduced, []byte(segment), 7)
	}
	reduced.Finish()

	require.Equal(t, raw, log.Bytes(), "the private log remains the complete source of truth")
	output := terminal.String()
	require.Contains(t, output, "5 dependencies will also be installed")
	require.Contains(t, output, "extra/python-build -> 1.4.3-1")
	require.Contains(t, output, "Diffs to show?")
	require.Contains(t, output, "diff --git a/PKGBUILD b/PKGBUILD")
	require.Contains(t, output, "PKGBUILDs to edit?")
	require.Contains(t, output, "Skipping verification of source file PGP signatures")
	require.Equal(t, 1, bytes.Count(terminal.Bytes(), []byte("Skipping verification")))
	require.Contains(t, output, "Proceed with installation? [Y/n]")
	require.Contains(t, output, "Packages (2) python-steam python-gevent-eventemitter")
	require.Contains(t, output, "Total Installed Size: 21.09 MiB")

	for _, noise := range []string{
		"% Total % Received", "659.5k", "steam.tar.gz ... Passed", "running egg_info",
		"copying hundreds", "55 passed", "python-steam downloading", "checking keyring",
		"checking package integrity", "loading package files", "checking for file conflicts",
		"checking available disk space",
	} {
		require.NotContains(t, output, noise)
	}
}

func TestAUROutput_newlineTerminatedApprovalEndsInstallPlan(t *testing.T) {
	var log bytes.Buffer
	var terminal bytes.Buffer
	reduced := newAUROutput(&log, Renderer{Out: &terminal})
	raw := "Packages (1) cursor-bin-3.16.17-1\n" +
		"Total Installed Size: 545.19 MiB\n" +
		":: Proceed with installation? [Y/n]\n" +
		"checking keyring...\n" +
		"checking package integrity...\n"
	_, err := io.WriteString(reduced, raw)
	require.NoError(t, err)
	reduced.Finish()

	require.Equal(t, raw, log.String())
	require.Contains(t, terminal.String(), "Packages (1) cursor-bin-3.16.17-1")
	require.Contains(t, terminal.String(), "Proceed with installation? [Y/n]")
	require.NotContains(t, terminal.String(), "checking keyring")
	require.NotContains(t, terminal.String(), "checking package integrity")
}

func TestAUROutput_promotesErrors(t *testing.T) {
	var log bytes.Buffer
	var terminal bytes.Buffer
	reduced := newAUROutput(&log, Renderer{Out: &terminal})
	raw := "compiler chatter\n==> ERROR: A failure occurred in build().\n"
	_, err := io.WriteString(reduced, raw)
	require.NoError(t, err)
	reduced.Finish()

	require.Equal(t, raw, log.String())
	require.Contains(t, terminal.String(), "✗ A failure occurred in build().")
	require.NotContains(t, terminal.String(), "compiler chatter")
}

func writeInChunks(t *testing.T, w io.Writer, p []byte, size int) {
	t.Helper()
	for len(p) > 0 {
		n := min(size, len(p))
		written, err := w.Write(p[:n])
		require.NoError(t, err)
		require.Equal(t, n, written)
		p = p[n:]
	}
}
