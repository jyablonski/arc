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
		RunInteractive: func(name string, args ...string) error { return nil },
		RunLogged:      func(io.Writer, bool, string, ...string) error { return nil },
		// Never reach the real yay from a test.
		RunAUR:            func(io.Writer, *os.File, string, ...string) error { return nil },
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

func TestRunWithDeps_defaultDoesNotCreatePersistentLog(t *testing.T) {
	deps := testDepsKernelStable(t)
	created := false
	deps.NewLog = func(time.Time) (*runLog, error) {
		created = true
		return nil, errors.New("must not create a log")
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.False(t, created)
}

func TestPackageVersionResult_unavailableDoesNotClaimUpdate(t *testing.T) {
	detail, changed := packageVersionResult("archlinux-keyring", nil, nil, errors.New("query failed"), nil)
	require.Equal(t, "status unavailable", detail)
	require.False(t, changed)
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

func TestRunWithDeps_failureTailWithoutPersistentLog(t *testing.T) {
	deps := testDepsKernelStable(t)
	var out bytes.Buffer
	deps.Out = &out
	deps.RunLogged = func(log io.Writer, _ bool, name string, args ...string) error {
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Sy" {
			_, writeErr := io.WriteString(log, "keyring diagnostic detail\n")
			require.NoError(t, writeErr)
			return errors.New("keyring failed")
		}
		return nil
	}

	err := RunWithDeps(deps, Options{})
	require.ErrorContains(t, err, "keyring update failed")
	require.Contains(t, out.String(), "subprocess output:")
	require.Contains(t, out.String(), "keyring diagnostic detail")
	require.NotContains(t, out.String(), "\n  log ")
}
