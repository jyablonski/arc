package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyablonski/arc/cmd"
	"github.com/jyablonski/arc/internal/extracmd"
	"github.com/stretchr/testify/require"
)

func resetExtraCommandsEnv(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv(extracmd.EnvVar)
	require.NoError(t, os.Unsetenv(extracmd.EnvVar))
	extracmd.ApplyVisibility()
	t.Cleanup(func() {
		if had {
			require.NoError(t, os.Setenv(extracmd.EnvVar, old))
		} else {
			require.NoError(t, os.Unsetenv(extracmd.EnvVar))
		}
		extracmd.ApplyVisibility()
	})
}

func TestExecute_help(t *testing.T) {
	resetExtraCommandsEnv(t)
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"arc", "help"}
	cmd.Execute()
}

func TestArcBinary_smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skips go build subprocess")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "arc")
	buildOut, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, string(buildOut))

	out, err := exec.Command(bin, "help").CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, strings.ToLower(string(out)), "arc")

	out, err = exec.Command(bin, "--help").CombinedOutput()
	require.NoError(t, err, string(out))
}
