package sysupdate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func testDepsKernelStable(t *testing.T) Deps {
	t.Helper()
	kernel := map[string]string{"linux": "1:6.6.1-1"}
	logDir := t.TempDir()
	return Deps{
		CheckPacman: func() error { return nil },
		KernelVersions: func() (map[string]string, error) {
			return kernel, nil
		},
		RunInteractive:    func(name string, args ...string) error { return nil },
		RunLogged:         func(io.Writer, bool, string, ...string) error { return nil },
		CheckYayAvailable: func() bool { return false },
		Stdin:             stdinWith(t, "\n"),
		Out:               io.Discard,
		Now:               time.Now,
		NewLog: func(now time.Time) (*runLog, error) {
			return newRunLogIn(logDir, now)
		},
		RepoPlan: func() ([]PackageChange, error) { return nil, nil },
		InstalledVersions: func() (map[string]string, error) {
			return map[string]string{"archlinux-keyring": "20260727-1"}, nil
		},
		// Keep AUR review offline by default; empty install set short-circuits
		// runAURReview before any network call.
		ForeignPackages: func() (map[string]string, error) { return map[string]string{}, nil },
		IgnoredPackages: func() ([]string, error) { return nil, nil },
	}
}

func TestRunWithDeps_success_skipAUR_skipCache(t *testing.T) {
	var logged [][]string
	deps := testDepsKernelStable(t)
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		logged = append(logged, append([]string{name}, args...))
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.Equal(t, [][]string{
		{"sudo", "-v"},
		{"sudo", "pacman", "-Sy", "--needed", "--noconfirm", "--noprogressbar", "--color", "never", "archlinux-keyring"},
	}, logged)
}

func TestPackageVersionResult_unavailableDoesNotClaimUpdate(t *testing.T) {
	require.Equal(t, "status unavailable", packageVersionResult("archlinux-keyring", nil, nil, errors.New("query failed"), nil))
}

func TestRunWithDeps_outputContract(t *testing.T) {
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.Stdin = stdinWith(t, "y\n")
	started := time.Date(2026, 8, 13, 7, 51, 25, 0, time.Local)
	times := []time.Time{started, started, started.Add(1400 * time.Millisecond), started.Add(2 * time.Second), started.Add(2400 * time.Millisecond)}
	deps.Now = func() time.Time {
		now := times[0]
		times = times[1:]
		return now
	}
	logFile, err := os.CreateTemp(t.TempDir(), "update-output-*.log")
	require.NoError(t, err)
	deps.NewLog = func(time.Time) (*runLog, error) {
		return &runLog{file: logFile, path: "/state/arc/update-output.log"}, nil
	}
	deps.RepoPlan = func() ([]PackageChange, error) {
		return []PackageChange{{Name: "bolt", FromVersion: "0.9.11-2", ToVersion: "0.9.11-3", SizeBytes: 1024 * 1024}}, nil
	}
	versionCalls := 0
	deps.InstalledVersions = func() (map[string]string, error) {
		versionCalls++
		bolt := "0.9.11-2"
		if versionCalls == 3 {
			bolt = "0.9.11-3"
		}
		return map[string]string{"archlinux-keyring": "20260727-1", "bolt": bolt}, nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.Equal(t, ""+
		"arc update system                                        2026-08-13 07:51:25\n"+
		"────────────────────────────────────────────────────────────────────────────\n\n"+
		"SYNC\n"+
		"  ✓ databases                synchronized                               1.4s\n"+
		"  ✓ archlinux-keyring        20260727-1 (current)\n\n"+
		"REPO                                                                1 update\n"+
		"  bolt  0.9.11-2 → 0.9.11-3\n\n"+
		"  download 1.0 MiB\n\n"+
		"  Proceed with 1 repo upgrade? [Y/n] \n"+
		"  ✓ transaction              1 package, 1.0 MiB                         0.4s\n"+
		"  ✓ verified                 installed package state\n"+
		"  ✓ bolt                     0.9.11-3\n"+
		"  ✓ hooks                    post-transaction complete\n\n"+
		"  log /state/arc/update-output.log\n", out.String())
}

func TestRunWithDeps_paccacheFailsContinues(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) > 0 && args[0] == "paccache" {
			return errors.New("paccache denied")
		}
		return nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true}))
}

func TestRunWithDeps_pacmanMissing(t *testing.T) {
	deps := Deps{
		CheckPacman: func() error {
			return shell.NewErrToolNotAvailable("pacman")
		},
	}
	err := RunWithDeps(deps, Options{})
	require.Error(t, err)
	var ta *shell.ErrToolNotAvailable
	require.ErrorAs(t, err, &ta)
	require.Equal(t, "pacman", ta.Tool)
}

func TestRunWithDeps_keyringFails(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.RunLogged = func(_ io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Sy" {
			return errors.New("keyring failed")
		}
		return nil
	}

	err := RunWithDeps(deps, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "keyring update failed")
}
