package sysupdate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/stretchr/testify/require"
)

func testPendingAURResult() *aurreview.Result {
	return &aurreview.Result{Updates: []aurreview.Update{{
		Name: "foo", InstalledVersion: "1.0", TargetVersion: "2.0",
	}}}
}

func testForeignPackageUpgrade() func() (map[string]string, error) {
	calls := 0
	return func() (map[string]string, error) {
		calls++
		if calls > 1 {
			return map[string]string{"foo": "2.0"}, nil
		}
		return map[string]string{"foo": "1.0"}, nil
	}
}

func TestRunWithDeps_yayPathAndPaccache(t *testing.T) {
	var yayCall []string
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = testForeignPackageUpgrade()
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	deps.CommitAUR = func(*aurreview.Result) error { return nil }
	var calls [][]string
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	deps.RunAUR = func(out io.Writer, _ *os.File, name string, args ...string) error {
		yayCall = append([]string{name}, args...)
		_, err := io.WriteString(out, "==> WARNING: captured warning\nnoisy build output\n")
		return err
	}
	deps.Stdin = stdinWith(t, "y\n")

	require.NoError(t, RunWithDeps(deps, Options{}))
	require.Equal(t, []string{
		"yay", "-Syu", "--aur",
		"--answerupgrade", "None",
		"--cleanmenu=false",
		"--editmenu=false",
		// arc reviewed the build files itself, so yay's diff menu stays off.
		"--diffmenu=false",
	}, yayCall)
	require.Contains(t, calls, []string{"sudo", "paccache", "-rv"})
	require.Contains(t, out.String(), "captured warning")
	require.NotContains(t, out.String(), "noisy build output")
	require.Contains(t, out.String(), "Upgrade 1 AUR package? [Y/n]")
}

func TestRunWithDeps_aurDeclinedSkipsYay(t *testing.T) {
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.Stdin = stdinWith(t, "n\n")
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = testForeignPackageUpgrade()
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	ranYay, committed := false, false
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error { ranYay = true; return nil }
	deps.CommitAUR = func(*aurreview.Result) error { committed = true; return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.False(t, ranYay)
	require.False(t, committed, "a declined upgrade must not advance the trusted baseline")
	require.Contains(t, out.String(), "AUR upgrade skipped")
	require.Contains(t, out.String(), "AUR declined")
}

func TestRunWithDeps_highFindingFlipsThePromptDefault(t *testing.T) {
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.Stdin = stdinWith(t, "\n")
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = testForeignPackageUpgrade()
	res := testPendingAURResult()
	res.Findings = []aurreview.Finding{{Pkg: "foo", Severity: aurreview.High, Message: "pipe-to-shell", Location: "PKGBUILD:3"}}
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) { return res, nil }
	ranYay := false
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error { ranYay = true; return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.Contains(t, out.String(), "Upgrade 1 AUR package? [y/N]")
	require.False(t, ranYay, "Enter declines when the review found a high-signal change")
}

func TestPrintAURReview_hidesInformationalNoise(t *testing.T) {
	var out bytes.Buffer
	result := &aurreview.Result{
		Updates: []aurreview.Update{{Name: "spotify", PackageBase: "spotify", InstalledVersion: "1", TargetVersion: "2"}},
		Findings: []aurreview.Finding{
			{Pkg: "spotify", Severity: aurreview.Info, Message: "unencrypted source URL", Location: "PKGBUILD:10"},
			{Pkg: "spotify", Severity: aurreview.Info, Message: "SKIP checksum on 3 lines", Location: "PKGBUILD:20"},
		},
	}

	printAURReview(Renderer{Out: &out, Width: 76}, result, nil, time.Unix(0, 0), false)

	require.Contains(t, out.String(), "no build logic changed")
	require.NotContains(t, out.String(), "unencrypted source URL")
	require.NotContains(t, out.String(), "SKIP checksum")
}

func TestPrintAURReview_keepsSuspiciousFindings(t *testing.T) {
	var out bytes.Buffer
	result := &aurreview.Result{
		Updates: []aurreview.Update{{Name: "foo", PackageBase: "foo", InstalledVersion: "1", TargetVersion: "2"}},
		Findings: []aurreview.Finding{
			{Pkg: "foo", Severity: aurreview.High, Message: "pipe-to-shell", Location: "PKGBUILD:10"},
			{Pkg: "foo", Severity: aurreview.Warn, Message: "runtime network download", Location: "PKGBUILD:10"},
		},
	}

	printAURReview(Renderer{Out: &out, Width: 76}, result, nil, time.Unix(0, 0), false)

	require.Contains(t, out.String(), "2 suspicious changes detected")
	require.Contains(t, out.String(), "✗ foo: pipe-to-shell (PKGBUILD:10)")
	require.Contains(t, out.String(), "⚠ foo: runtime network download (PKGBUILD:10)")
}

func TestRunWithDeps_aurReviewCommitsOnYaySuccess(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	foreignCalls := 0
	deps.ForeignPackages = func() (map[string]string, error) {
		foreignCalls++
		version := "1.0"
		if foreignCalls > 1 {
			version = "2.0"
		}
		return map[string]string{"foo": version}, nil
	}
	res := testPendingAURResult()
	var reviewed bool
	deps.ReviewAUR = func(_ context.Context, installed map[string]string) (*aurreview.Result, error) {
		reviewed = true
		require.Equal(t, "1.0", installed["foo"])
		return res, nil
	}
	var committed *aurreview.Result
	deps.CommitAUR = func(r *aurreview.Result) error { committed = r; return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.True(t, reviewed)
	require.Same(t, res, committed)
}

func TestRunWithDeps_aurReviewExcludesIgnoredPackages(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = func() (map[string]string, error) {
		return map[string]string{"spotify": "1.0", "foo": "1.0", "linux-custom": "6.0"}, nil
	}
	deps.IgnoredPackages = func() ([]string, error) {
		return []string{"spotify", "linux-*"}, nil
	}
	var reviewed map[string]string
	var ranYay bool
	var committed *aurreview.Result
	deps.ReviewAUR = func(_ context.Context, installed map[string]string) (*aurreview.Result, error) {
		reviewed = installed
		return &aurreview.Result{}, nil
	}
	deps.CommitAUR = func(result *aurreview.Result) error { committed = result; return nil }
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error { ranYay = true; return nil }
	var out bytes.Buffer
	deps.Out = &out

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.Equal(t, map[string]string{"foo": "1.0"}, reviewed)
	require.False(t, ranYay)
	require.NotNil(t, committed, "a successful current-state review must advance the provenance baseline")
	require.Contains(t, out.String(), "no eligible updates")
	require.NotContains(t, out.String(), "yay will retain")
}

func TestRunWithDeps_allAURPackagesIgnoredSkipsYay(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = func() (map[string]string, error) {
		return map[string]string{"spotify": "1.0"}, nil
	}
	deps.IgnoredPackages = func() ([]string, error) { return []string{"spotify"}, nil }
	var out bytes.Buffer
	deps.Out = &out
	var ranYay bool
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error { ranYay = true; return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.False(t, ranYay)
	require.Contains(t, out.String(), "· spotify             held at 1.0")
	require.Contains(t, out.String(), "no eligible updates")
}

func TestRunWithDeps_aurReviewNoCommitOnYayFailure(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = func() (map[string]string, error) {
		return map[string]string{"foo": "1.0"}, nil
	}
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	var committed bool
	deps.CommitAUR = func(*aurreview.Result) error { committed = true; return nil }
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error { return errors.New("boom") }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.False(t, committed)
}

func TestRunWithDeps_aurReviewNoCommitWhenYayExitsWithoutApplyingPlan(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = func() (map[string]string, error) {
		return map[string]string{"foo": "1.0"}, nil
	}
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	var committed bool
	deps.CommitAUR = func(*aurreview.Result) error { committed = true; return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.False(t, committed)
}

func TestRunWithDeps_yayUnavailableMessage(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return false }
	deps.RunInteractive = func(name string, args ...string) error { return nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
}

func TestRunWithDeps_yayFailsContinues(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.ForeignPackages = func() (map[string]string, error) {
		return map[string]string{"foo": "1.0"}, nil
	}
	deps.ReviewAUR = func(context.Context, map[string]string) (*aurreview.Result, error) {
		return testPendingAURResult(), nil
	}
	var ranYay bool
	deps.RunAUR = func(io.Writer, *os.File, string, ...string) error {
		ranYay = true
		return errors.New("yay boom")
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.True(t, ranYay)
}

func TestAURResultMismatches_regularAndVCS(t *testing.T) {
	result := &aurreview.Result{Updates: []aurreview.Update{
		{Name: "cursor-bin", InstalledVersion: "1", TargetVersion: "2"},
		{Name: "tool-git", InstalledVersion: "r10", TargetVersion: "r11"},
	}}

	require.Empty(t, aurResultMismatches(result, map[string]string{"cursor-bin": "2", "tool-git": "r12"}))
	require.Equal(t, []string{
		"cursor-bin planned 2, installed 1",
		"tool-git remained at r10",
	}, aurResultMismatches(result, map[string]string{"cursor-bin": "1", "tool-git": "r10"}))
}

func TestPublishedAgo(t *testing.T) {
	now := time.Unix(1_000_000_000, 0)
	require.Equal(t, "", publishedAgo(now, 0))
	require.Equal(t, "28m ago", publishedAgo(now, now.Add(-28*time.Minute).Unix()))
	require.Equal(t, "6h ago", publishedAgo(now, now.Add(-6*time.Hour).Unix()))
	require.Equal(t, "3d ago", publishedAgo(now, now.Add(-72*time.Hour).Unix()))
}
