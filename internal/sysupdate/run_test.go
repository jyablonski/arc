package sysupdate

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func testDepsKernelStable(t *testing.T) Deps {
	t.Helper()
	kernel := map[string]string{"linux": "1:6.6.1-1"}
	return Deps{
		CheckPacman: func() error { return nil },
		KernelVersions: func() (map[string]string, error) {
			return kernel, nil
		},
		RunInteractive: func(name string, args ...string) error { return nil },
		RunSudo: func(name string, args ...string) (string, error) {
			return "", nil
		},
		CheckYayAvailable: func() bool { return false },
	}
}

func TestRunWithDeps_success_skipAUR_skipCache(t *testing.T) {
	var interactive [][]string
	var sudo [][]string
	deps := testDepsKernelStable(t)
	deps.RunInteractive = func(name string, args ...string) error {
		interactive = append(interactive, append([]string{name}, args...))
		return nil
	}
	deps.RunSudo = func(name string, args ...string) (string, error) {
		sudo = append(sudo, append([]string{name}, args...))
		return "", nil
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
	require.Len(t, interactive, 2)
	require.Equal(t, []string{"sudo", "pacman", "-Sy", "--needed", "--noconfirm", "archlinux-keyring"}, interactive[0])
	require.Equal(t, []string{"sudo", "pacman", "-Syu", "--noconfirm"}, interactive[1])
	require.Empty(t, sudo)
}

func TestRunWithDeps_yayPathAndPaccache(t *testing.T) {
	var sawYay bool
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.RunInteractive = func(name string, args ...string) error {
		if name == "yay" {
			sawYay = true
		}
		return nil
	}
	var paccache [][]string
	deps.RunSudo = func(name string, args ...string) (string, error) {
		paccache = append(paccache, append([]string{name}, args...))
		return "", nil
	}

	require.NoError(t, RunWithDeps(deps, Options{}))
	require.True(t, sawYay)
	require.Len(t, paccache, 1)
	require.Equal(t, []string{"paccache", "-rv"}, paccache[0])
}

func TestRunWithDeps_yayUnavailableMessage(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return false }
	deps.RunInteractive = func(name string, args ...string) error { return nil }
	deps.RunSudo = func(string, ...string) (string, error) { return "", nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
}

func TestRunWithDeps_yayFailsContinues(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.CheckYayAvailable = func() bool { return true }
	deps.RunInteractive = func(name string, args ...string) error {
		if name == "yay" {
			return errors.New("yay boom")
		}
		return nil
	}
	deps.RunSudo = func(string, ...string) (string, error) { return "", nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipCache: true}))
}

func TestRunWithDeps_paccacheFailsContinues(t *testing.T) {
	deps := testDepsKernelStable(t)
	deps.RunInteractive = func(string, ...string) error { return nil }
	deps.RunSudo = func(string, ...string) (string, error) {
		return "", errors.New("paccache denied")
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true}))
}

func TestRunWithDeps_kernelBump_promptRebootNo(t *testing.T) {
	f, err := os.CreateTemp("", "sysupdate-stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("n\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	before := map[string]string{"linux": "1:6.6.1-1"}
	after := map[string]string{"linux": "1:6.6.2-1"}
	calls := 0
	deps := Deps{
		CheckPacman: func() error { return nil },
		KernelVersions: func() (map[string]string, error) {
			if calls == 0 {
				calls++
				return before, nil
			}
			return after, nil
		},
		RunInteractive:    func(string, ...string) error { return nil },
		RunSudo:           func(string, ...string) (string, error) { return "", nil },
		CheckYayAvailable: func() bool { return false },
		Stdin:             f,
	}

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
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
	deps.RunInteractive = func(name string, args ...string) error {
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Sy" {
			return errors.New("keyring failed")
		}
		return nil
	}

	err := RunWithDeps(deps, Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "keyring update failed")
}

func TestRunWithDeps_syuFails(t *testing.T) {
	deps := testDepsKernelStable(t)
	var sawKeyring bool
	deps.RunInteractive = func(name string, args ...string) error {
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Sy" {
			sawKeyring = true
			return nil
		}
		if name == "sudo" && len(args) >= 2 && args[0] == "pacman" && args[1] == "-Syu" {
			return errors.New("syu failed")
		}
		return nil
	}

	err := RunWithDeps(deps, Options{})
	require.True(t, sawKeyring)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pacman update failed")
}

func TestRunWithDeps_kernelVersionsWarnBefore(t *testing.T) {
	f, err := os.CreateTemp("", "sysupdate-kern-warn")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("n\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	calls := 0
	deps := testDepsKernelStable(t)
	deps.Stdin = f
	deps.KernelVersions = func() (map[string]string, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("rpm db locked")
		}
		return map[string]string{"linux": "1:1-1"}, nil
	}
	deps.RunInteractive = func(string, ...string) error { return nil }
	deps.RunSudo = func(string, ...string) (string, error) { return "", nil }

	require.NoError(t, RunWithDeps(deps, Options{SkipAUR: true, SkipCache: true}))
}

func TestPromptReboot_readError(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())

	err = promptReboot(r, func(string, ...string) error { return nil })
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "read")
	_ = r.Close()
}

func TestPromptReboot_sudoRebootFails(t *testing.T) {
	f, err := os.CreateTemp("", "stdin-y")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("y\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	err = promptReboot(f, func(string, ...string) error {
		return errors.New("permission denied")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to reboot")
}
