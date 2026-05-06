package sysupdate

import (
	"fmt"
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestPromptReboot_skipsWhenNo(t *testing.T) {
	f, err := os.CreateTemp("", "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString("n\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	shell.SetMockRunner(&shell.MockRunner{
		RunInteractiveFunc: func(name string, args ...string) error {
			require.Fail(t, "unexpected interactive run", "%s %v", name, args)
			return nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, PromptReboot(f))
}

func TestPromptReboot_runsRebootWhenYes(t *testing.T) {
	f, err := os.CreateTemp("", "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	_, err = f.WriteString("y\n")
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	var saw bool
	shell.SetMockRunner(&shell.MockRunner{
		RunInteractiveFunc: func(name string, args ...string) error {
			if name == "sudo" && len(args) >= 1 && args[0] == "reboot" {
				saw = true
				return nil
			}
			return fmt.Errorf("unexpected: %s %v", name, args)
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	require.NoError(t, PromptReboot(f))
	require.True(t, saw)
}
