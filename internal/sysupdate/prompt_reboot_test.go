package sysupdate

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stdinWith writes body to a temp file rewound to the start, for use as the
// promptReboot stdin.
func stdinWith(t *testing.T, body string) *os.File {
	t.Helper()
	f, err := os.CreateTemp("", "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString(body)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	return f
}

func TestPromptReboot_skipsWhenNo(t *testing.T) {
	f := stdinWith(t, "n\n")

	runInteractive := func(name string, args ...string) error {
		require.Fail(t, "unexpected interactive run", "%s %v", name, args)
		return nil
	}

	require.NoError(t, promptReboot(f, runInteractive))
}

func TestPromptReboot_runsRebootWhenYes(t *testing.T) {
	f := stdinWith(t, "y\n")

	var saw bool
	runInteractive := func(name string, args ...string) error {
		if name == "sudo" && len(args) >= 1 && args[0] == "reboot" {
			saw = true
			return nil
		}
		return fmt.Errorf("unexpected: %s %v", name, args)
	}

	require.NoError(t, promptReboot(f, runInteractive))
	require.True(t, saw)
}

func TestPromptReboot_readError(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	t.Cleanup(func() { _ = r.Close() })

	err = promptReboot(r, func(string, ...string) error { return nil })
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "read")
}

func TestPromptReboot_sudoRebootFails(t *testing.T) {
	f := stdinWith(t, "y\n")

	err := promptReboot(f, func(string, ...string) error {
		return errors.New("permission denied")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to reboot")
}
