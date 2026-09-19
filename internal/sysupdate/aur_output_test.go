package sysupdate

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// proxy builds an aurOutput whose person-side input is typed and whose
// yay-side answers are captured.
func proxy(t *testing.T, renderer Renderer, typed string, opts aurOutputOptions) (*aurOutput, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var log, answers bytes.Buffer
	opts.answers = &answers
	opts.input = bufio.NewReader(strings.NewReader(typed))
	return newAUROutput(&log, renderer, opts), &log, &answers
}

func approvedSet(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

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
			"==> A\n",
		"diff --git /home/test/.cache/yay/python-steam/PKGBUILD /home/test/.cache/yay/python-steam/PKGBUILD\n-pkgrel=2\n+pkgrel=3\n" +
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
			"==> WARNING: Using existing $srcdir/ tree\n",
		"Packages (2) python-steam-2.0.0.alpha1-3 python-gevent-eventemitter-2.1-6\n" +
			"Total Download Size: 3.40 MiB\n" +
			"Total Installed Size: 21.09 MiB\n\n" +
			":: Proceed with installation? [Y/n] ",
		"\n:: Retrieving packages...\n" +
			"python-steam downloading...\n" +
			"checking package integrity...\n" +
			":: Processing package changes...\n" +
			"upgrading python-steam...\n",
	}
	raw := []byte(strings.Join(segments, ""))

	var terminal bytes.Buffer
	r := NewRenderer(&terminal, false)
	// No arc approval (review unavailable): pacman's gate is put to the person.
	reduced, log, answers := proxy(t, r, "y\n", aurOutputOptions{diffPackages: []string{"python-gevent-eventemitter", "python-steam"}})
	for _, segment := range segments {
		writeInChunks(t, reduced, []byte(segment), 7)
	}
	reduced.Finish()

	require.Equal(t, "y\n", answers.String())
	require.Equal(t, raw, log.Bytes(), "the private log remains the complete source of truth")
	output := terminal.String()
	require.Contains(t, output, "5 dependencies will also be installed")
	require.Contains(t, output, "extra/python-build -> 1.4.3-1")
	require.NotContains(t, output, "Packages to exclude:")
	require.NotContains(t, output, "Diffs to show?")
	require.Contains(t, output, "diff · python-steam")
	require.Contains(t, output, "-pkgrel=2")
	require.Contains(t, output, "+pkgrel=3")
	require.NotContains(t, output, "diff --git")
	require.NotContains(t, output, "PKGBUILDs to edit?")
	require.NotContains(t, output, "[N]one [A]ll")
	require.NotContains(t, output, "Skipping verification", "routine warnings are only counted")
	require.Equal(t, 1, r.tally.warnings, "duplicate warnings count once")
	require.Equal(t, 1, r.tally.hidden)
	require.NotContains(t, output, "Using existing $srcdir/ tree")
	require.Contains(t, output, "Packages (2) python-steam-2.0.0.alpha1-3 python-gevent-eventemitter-2.1-6")
	require.Contains(t, output, "Install the packages above? [Y/n]")
	require.NotContains(t, output, "Proceed with installation?", "gates are reworded in arc's voice")

	for _, noise := range []string{
		"% Total % Received", "659.5k", "steam.tar.gz ... Passed", "running egg_info",
		"copying hundreds", "55 passed", "python-steam downloading", "checking package integrity",
	} {
		require.NotContains(t, output, noise)
	}
}

func TestAUROutput_verboseShowsRoutineWarnings(t *testing.T) {
	var terminal bytes.Buffer
	r := NewRenderer(&terminal, true)
	reduced, _, _ := proxy(t, r, "", aurOutputOptions{})
	_, err := io.WriteString(reduced, "==> Making package: foo 1-1\n==> WARNING: Skipping verification of source file PGP signatures.\n")
	require.NoError(t, err)
	reduced.Finish()
	require.Contains(t, terminal.String(), "⚠ foo: Skipping verification of source file PGP signatures.")
	require.Equal(t, 0, r.tally.hidden)
}

func TestAUROutput_answersGatesThatMatchTheApprovedPlan(t *testing.T) {
	var terminal bytes.Buffer
	clock := time.Unix(0, 0)
	reduced, log, answers := proxy(t, NewRenderer(&terminal, false), "", aurOutputOptions{
		approved: approvedSet("cursor-bin", "google-cloud-cli"),
		now:      func() time.Time { return clock },
	})
	write := func(s string) { writeInChunks(t, reduced, []byte(s), 9) }

	write("1 aur/cursor-bin 3.21.13-1 -> 3.21.16-1\n:: Proceed with install? [Y/n] ")
	write("\n==> Making package: cursor-bin 3.21.16-1 (Fri)\n")
	clock = clock.Add(38 * time.Second)
	write("==> Making package: google-cloud-cli 585.0.0-1 (Fri)\n")
	clock = clock.Add(12 * time.Second)
	write("Packages (3) cursor-bin-3.21.16-1  google-cloud-cli-585.0.0-1\n" +
		"             google-cloud-cli-debug-585.0.0-1\n\n" +
		"Total Installed Size:  691.51 MiB\nNet Upgrade Size:        2.87 MiB\n\n" +
		":: Proceed with installation? [Y/n] ")
	reduced.Finish()

	require.Equal(t, "y\ny\n", answers.String())
	require.Empty(t, terminal.String(), "matching gates are answered silently")
	require.Contains(t, log.String(), `[arc] answered "y": pacman transaction matches the approved plan`)
	require.Equal(t, map[string]time.Duration{"cursor-bin": 38 * time.Second, "google-cloud-cli": 12 * time.Second}, reduced.BuildTimes())
	require.Equal(t, int64(3009413), reduced.NetBytes(), "2.87 MiB")
}

func TestAUROutput_surfacesAURPackagesOutsideThePlan(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, answers := proxy(t, NewRenderer(&terminal, false), "n\n", aurOutputOptions{approved: approvedSet("foo")})
	_, err := io.WriteString(reduced, "AUR Dependency (1): evil-lib-1.0-1\n:: Proceed with install? [Y/n] ")
	require.NoError(t, err)

	require.Equal(t, "n\n", answers.String())
	require.Contains(t, terminal.String(), "⚠ yay also wants to build 1 AUR package not in the approved plan: evil-lib")
	require.Contains(t, terminal.String(), "Build them too? [Y/n] ")
}

func TestAUROutput_surfacesPacmanTransactionOutsideThePlan(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, answers := proxy(t, NewRenderer(&terminal, false), "\n", aurOutputOptions{approved: approvedSet("foo")})
	_, err := io.WriteString(reduced, "Packages (2) python-build-1.4.3-1  python-wheel-0.45-1\n\n:: Proceed with installation? [Y/n] ")
	require.NoError(t, err)

	require.Equal(t, "\n", answers.String(), "an empty answer lets pacman apply the default it showed")
	require.Contains(t, terminal.String(), "· pacman wants to install 2 packages not in the approved plan: python-build, python-wheel")
	require.Contains(t, terminal.String(), "Install them? [Y/n] ")
}

func TestAUROutput_ordinaryOutputIsNotTreatedAsAPrompt(t *testing.T) {
	reduced, _, answers := proxy(t, NewRenderer(io.Discard, false), "typed\n", aurOutputOptions{})
	_, err := io.WriteString(reduced, "   CC libfoo/bar.o:")
	require.NoError(t, err)
	time.Sleep(unknownPromptDelay + 500*time.Millisecond)
	reduced.mu.Lock()
	defer reduced.mu.Unlock()
	require.Empty(t, answers.String(), "a build pausing mid-line must not consume the person's input")
}

func TestAUROutput_unknownPromptIsForwardedVerbatim(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, answers := proxy(t, NewRenderer(&terminal, false), "2\n", aurOutputOptions{approved: approvedSet("foo")})
	_, err := io.WriteString(reduced, ":: There are 2 providers available for java-runtime:\n    1) jre-openjdk 2) jre17\n\nEnter a number (default=1): ")
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		reduced.mu.Lock()
		defer reduced.mu.Unlock()
		return answers.String() == "2\n"
	}, 2*unknownPromptDelay, 20*time.Millisecond, "question-shaped output must not deadlock the proxied stdin")
	reduced.Finish()
	require.Contains(t, terminal.String(), "Enter a number (default=1): ")
}

func TestAUROutput_closedInputRefuses(t *testing.T) {
	reduced, _, answers := proxy(t, NewRenderer(io.Discard, false), "", aurOutputOptions{})
	_, err := io.WriteString(reduced, ":: Proceed with install? [Y/n] ")
	require.NoError(t, err)
	require.Equal(t, "n\n", answers.String())
}

func TestAUROutput_diffEndsWithoutEditMenu(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, _ := proxy(t, Renderer{Out: &terminal}, "y\n", aurOutputOptions{diffPackages: []string{"foo"}})
	segments := []string{
		"1 foo (Installed)\n==> Diffs to show?\n==> [N]one [A]ll [Ab]ort or (1)\n==> A\n",
		"diff --git a/PKGBUILD b/PKGBUILD\n-pkgver=1\n+pkgver=2\n\n:: Proceed with install? [Y/n] ",
		":: Parsing SRCINFO: foo\nnoisy parser output\n",
	}
	for _, segment := range segments {
		writeInChunks(t, reduced, []byte(segment), 5)
	}
	reduced.Finish()

	output := terminal.String()
	require.NotContains(t, output, "Diffs to show?")
	require.Contains(t, output, "diff · foo")
	require.Contains(t, output, "-pkgver=1")
	require.Contains(t, output, "+pkgver=2")
	require.Contains(t, output, "Build the AUR packages above? [Y/n]")
	require.NotContains(t, output, "noisy parser output")
}

func TestAUROutput_groupsAndColorsEachPackageDiff(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	var terminal bytes.Buffer
	reduced, _, _ := proxy(t, Renderer{Out: &terminal, ForceColor: true}, "y\n", aurOutputOptions{diffPackages: []string{"bar", "foo"}})
	raw := "==> Diffs to show?\n" +
		"==> [N]one [A]ll [Ab]ort or (1 2)\n" +
		"==> A\n" +
		"diff --git /home/test/.cache/yay/foo/PKGBUILD /home/test/.cache/yay/foo/PKGBUILD\n" +
		"index 111..222 100644\n--- /home/test/.cache/yay/foo/PKGBUILD\n+++ /home/test/.cache/yay/foo/PKGBUILD\n" +
		"@@ -1 +1 @@\n-pkgver=1\n+pkgver=2\n" +
		"diff --git /home/test/.cache/yay/bar/bar.install /home/test/.cache/yay/bar/bar.install\n" +
		"@@ -1 +1 @@\n-old\n+new\n" +
		":: Proceed with install? [Y/n] "

	_, err := io.WriteString(reduced, raw)
	require.NoError(t, err)
	reduced.Finish()

	output := terminal.String()
	require.Contains(t, output, "diff · foo")
	require.Contains(t, output, "diff · bar")
	require.Contains(t, output, "bar.install")
	require.Contains(t, output, "\x1b[31m-pkgver=1\x1b[0m")
	require.Contains(t, output, "\x1b[32m+pkgver=2\x1b[0m")
	require.Less(t, strings.Index(output, "diff · foo"), strings.Index(output, "diff · bar"))
	require.Less(t, strings.Index(output, "+new"), strings.Index(output, "Build the AUR packages above?"))
}

func TestAUROutput_newlineTerminatedPromptIsNotAnswered(t *testing.T) {
	var terminal bytes.Buffer
	reduced, log, answers := proxy(t, Renderer{Out: &terminal}, "", aurOutputOptions{})
	raw := "Packages (1) cursor-bin-3.16.17-1\n" +
		"Total Installed Size: 545.19 MiB\n" +
		":: Proceed with installation? [Y/n]\n" +
		"checking keyring...\n" +
		"checking package integrity...\n"
	_, err := io.WriteString(reduced, raw)
	require.NoError(t, err)
	reduced.Finish()

	require.Equal(t, raw, log.String())
	require.Empty(t, answers.String(), "a terminated prompt is not waiting for input")
	require.NotContains(t, terminal.String(), "checking keyring")
	require.NotContains(t, terminal.String(), "checking package integrity")
}

func TestAUROutput_installPromptAfterDiffIsVisibleBeforeInput(t *testing.T) {
	for _, prompt := range []struct{ name, raw string }{
		{"plain", ":: Proceed with install? [Y/n] "},
		{"colored install", "\x1b[1m\x1b[36m:: \x1b[0m\x1b[1mProceed with install?\x1b[0m \x1b[1m[Y/n]\x1b[0m "},
	} {
		// yay leaves the prompt unterminated and blocks on stdin, so the gate has
		// to reach the terminal without a trailing newline to flush it.
		for _, size := range []int{1, 5, 4096} {
			t.Run(fmt.Sprintf("%s/chunk=%d", prompt.name, size), func(t *testing.T) {
				var terminal bytes.Buffer
				reduced, log, answers := proxy(t, Renderer{Out: &terminal}, "y\n", aurOutputOptions{diffPackages: []string{"foo"}})
				raw := "==> Diffs to show?\n==> [N]one [A]ll [Ab]ort\n==> A\n" +
					"diff --git a/PKGBUILD b/PKGBUILD\n-pkgver=1\n+pkgver=2\n" + prompt.raw
				writeInChunks(t, reduced, []byte(raw), size)

				want := "\n    diff · foo\n    PKGBUILD\n    -pkgver=1\n    +pkgver=2\n" +
					"  Build the AUR packages above? [Y/n] \n"
				require.Equal(t, want, terminal.String())
				require.Equal(t, raw, log.String())
				require.Equal(t, "y\n", answers.String())
				reduced.Finish()
				require.Equal(t, want, terminal.String())
			})
		}
	}
}

func TestAUROutput_countedSRCINFOEndsReview(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, _ := proxy(t, Renderer{Out: &terminal}, "", aurOutputOptions{diffPackages: []string{"foo"}})
	// The install prompt normally ends the review; SRCINFO parsing is the
	// fallback when it is skipped, and yay counts that line.
	raw := "==> Diffs to show?\n==> [N]one [A]ll [Ab]ort\n==> A\n" +
		"diff --git a/PKGBUILD b/PKGBUILD\n-pkgver=1\n+pkgver=2\n" +
		":: (1/1) Parsing SRCINFO: foo\nnoisy parser output\n"
	writeInChunks(t, reduced, []byte(raw), 5)
	reduced.Finish()

	require.Equal(t, "\n    diff · foo\n    PKGBUILD\n    -pkgver=1\n    +pkgver=2\n", terminal.String())
}

func TestAUROutput_promptTextInDiffRemainsVisible(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, _ := proxy(t, Renderer{Out: &terminal}, "y\n", aurOutputOptions{diffPackages: []string{"foo"}})
	raw := "==> Diffs to show?\n==> [N]one [A]ll [Ab]ort\n==> A\n" +
		"diff --git a/PKGBUILD b/PKGBUILD\n" +
		"-echo ':: Proceed with install? [Y/n]'\n" +
		"+echo '==> Making package: foo'\n" +
		" :: Proceed with install? [Y/n]\n" +
		"+pkgver=2\n" +
		"\x1b[36m:: \x1b[0mProceed with install? [Y/n] "
	writeInChunks(t, reduced, []byte(raw), 5)

	require.Equal(t, "\n    diff · foo\n    PKGBUILD\n"+
		"    -echo ':: Proceed with install? [Y/n]'\n"+
		"    +echo '==> Making package: foo'\n"+
		"     :: Proceed with install? [Y/n]\n"+
		"    +pkgver=2\n"+
		"  Build the AUR packages above? [Y/n] \n", terminal.String())
}

func TestAUROutput_promotesErrors(t *testing.T) {
	var terminal bytes.Buffer
	reduced, log, _ := proxy(t, Renderer{Out: &terminal}, "", aurOutputOptions{})
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

func TestAUROutput_unreadablePacmanPlanIsPutToThePerson(t *testing.T) {
	var terminal bytes.Buffer
	reduced, _, answers := proxy(t, NewRenderer(&terminal, false), "n\n", aurOutputOptions{approved: approvedSet("foo")})
	// No "Packages (N)" line arrived, so arc has nothing to compare against
	// and must not auto-approve on the strength of an empty package list.
	_, err := io.WriteString(reduced, ":: Proceed with installation? [Y/n] ")
	require.NoError(t, err)

	require.Equal(t, "n\n", answers.String())
	require.Contains(t, terminal.String(), "Install the packages above? [Y/n] ")
}
