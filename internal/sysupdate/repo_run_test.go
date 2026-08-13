package sysupdate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunWithDeps_repromptsWhenPlanChanges(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.Stdin = stdinWith(t, "y\ny\n")
	first := []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}
	second := []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "3"}}
	planCalls := 0
	deps.RepoPlan = func() ([]PackageChange, error) {
		planCalls++
		if planCalls == 1 {
			return first, nil
		}
		return second, nil
	}
	deps.InstalledVersions = func() (map[string]string, error) {
		return map[string]string{"archlinux-keyring": "1", "foo": "3"}, nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.Equal(t, 3, planCalls)
}

func TestRunWithDeps_revalidationCanBecomeUpToDate(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.Stdin = stdinWith(t, "y\n")
	var out bytes.Buffer
	deps.Out = &out
	planCalls := 0
	deps.RepoPlan = func() ([]PackageChange, error) {
		planCalls++
		if planCalls == 1 {
			return []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}, nil
		}
		return nil, nil
	}
	var applied bool
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) > 1 && args[0] == "pacman" && args[1] == "-Su" {
			applied = true
		}
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.False(t, applied)
	require.Contains(t, out.String(), "no updates remain")
}

func TestRunWithDeps_declineDoesNotApplyRepoPlan(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.Stdin = stdinWith(t, "n\n")
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}, nil
	}
	var applied bool
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) > 1 && args[0] == "pacman" && args[1] == "-Su" {
			applied = true
		}
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.False(t, applied)
}

func TestRunWithDeps_assumeYesDoesNotReadApproval(t *testing.T) {
	deps := testDepsKernelStable(t)
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	t.Cleanup(func() { _ = r.Close() })
	deps.Stdin = r
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}, nil
	}
	deps.InstalledVersions = func() (map[string]string, error) {
		return map[string]string{"archlinux-keyring": "1", "foo": "2"}, nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true, AssumeYes: true}))
}

func TestRunWithDeps_failsWhenInstalledStateDoesNotMatchPlan(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.Stdin = stdinWith(t, "y\n")
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}, nil
	}
	deps.InstalledVersions = func() (map[string]string, error) {
		return map[string]string{"archlinux-keyring": "1", "foo": "1"}, nil
	}

	err := RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true})
	require.ErrorContains(t, err, "result verification")
}

func TestRunWithDeps_syuFails(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.Stdin = stdinWith(t, "y\n")
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "foo", FromVersion: "1", ToVersion: "2"}}, nil
	}
	var sawKeyring bool
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Sy" {
			sawKeyring = true
			return nil
		}
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Su" {
			return errors.New("syu failed")
		}
		return nil
	}

	err := RunWithDeps(deps, Options{})
	require.True(t, sawKeyring)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pacman update failed")
}
