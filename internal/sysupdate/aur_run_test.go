package sysupdate

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	var yayVisible bool
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
	deps.RunLogged = func(log io.Writer, visible bool, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if name == "yay" {
			yayCall = append([]string{name}, args...)
			yayVisible = visible
			_, err := io.WriteString(log, "==> WARNING: captured warning\nnoisy build output\n")
			return err
		}
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{}))
	require.Equal(t, []string{"yay", "-Syu", "--aur", "--diffmenu", "--editmenu", "--noanswerdiff", "--noansweredit"}, yayCall)
	require.False(t, yayVisible, "yay output must pass through the reducer rather than mirror directly")
	require.Contains(t, calls, []string{"sudo", "paccache", "-rv"})
	require.Contains(t, out.String(), "captured warning")
	require.NotContains(t, out.String(), "noisy build output")
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
	deps.RunLogged = func(_ io.Writer, _ bool, name string, _ ...string) error {
		if name == "yay" {
			ranYay = true
		}
		return nil
	}
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
	deps.RunLogged = func(_ io.Writer, _ bool, name string, _ ...string) error {
		if name == "yay" {
			ranYay = true
		}
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
	require.False(t, ranYay)
	require.Contains(t, out.String(), "spotify 1.0 ignored by IgnorePkg")
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
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "yay" {
			return errors.New("boom")
		}
		return nil
	}

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
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "yay" {
			ranYay = true
			return errors.New("yay boom")
		}
		return nil
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
	require.Equal(t, "published 28m ago", publishedAgo(now, now.Add(-28*time.Minute).Unix()))
	require.Equal(t, "published 6h ago", publishedAgo(now, now.Add(-6*time.Hour).Unix()))
	require.Equal(t, "published 3d ago", publishedAgo(now, now.Add(-72*time.Hour).Unix()))
}
