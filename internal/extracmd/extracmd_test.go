package extracmd_test

import (
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/extracmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnabled(t *testing.T) {
	tests := []struct {
		name     string
		value    *string
		expected bool
	}{
		{name: "unset defaults to disabled", value: nil, expected: false},
		{name: "one enables extra commands", value: ptr("1"), expected: true},
		{name: "true enables extra commands", value: ptr("true"), expected: true},
		{name: "false disables extra commands", value: ptr("false"), expected: false},
		{name: "invalid value disables extra commands", value: ptr("gh restart-dashboard"), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue, hadValue := os.LookupEnv(extracmd.EnvVar)
			defer func() {
				if hadValue {
					_ = os.Setenv(extracmd.EnvVar, oldValue)
				} else {
					_ = os.Unsetenv(extracmd.EnvVar)
				}
			}()

			if tt.value == nil {
				_ = os.Unsetenv(extracmd.EnvVar)
			} else {
				_ = os.Setenv(extracmd.EnvVar, *tt.value)
			}

			assert.Equal(t, tt.expected, extracmd.Enabled())
		})
	}
}

func ptr(s string) *string {
	return &s
}

func TestNormalizeRelativePath(t *testing.T) {
	root := &cobra.Command{Use: "arc"}
	leaf := &cobra.Command{Use: "leaf"}
	deep := &cobra.Command{Use: "deep"}
	root.AddCommand(leaf)
	leaf.AddCommand(deep)

	require.Equal(t, "leaf", extracmd.NormalizeRelativePath(leaf))
	require.Equal(t, "leaf deep", extracmd.NormalizeRelativePath(deep))
}

func TestEnsureAvailable(t *testing.T) {
	root := &cobra.Command{Use: "arc"}
	cmd := &cobra.Command{Use: "secret"}
	root.AddCommand(cmd)

	oldVal, had := os.LookupEnv(extracmd.EnvVar)
	defer func() {
		if had {
			_ = os.Setenv(extracmd.EnvVar, oldVal)
		} else {
			_ = os.Unsetenv(extracmd.EnvVar)
		}
	}()

	_ = os.Unsetenv(extracmd.EnvVar)
	err := extracmd.EnsureAvailable(cmd)
	require.Error(t, err)
	require.Contains(t, err.Error(), `command "secret" is not available`)

	_ = os.Setenv(extracmd.EnvVar, "1")
	require.NoError(t, extracmd.EnsureAvailable(cmd))
}

func TestRegisterHiddenUnlessEnabled_and_ApplyVisibility(t *testing.T) {
	c := &cobra.Command{Use: "arc-extracmd-test-visibility"}
	extracmd.RegisterHiddenUnlessEnabled(c)

	oldVal, had := os.LookupEnv(extracmd.EnvVar)
	defer func() {
		if had {
			_ = os.Setenv(extracmd.EnvVar, oldVal)
		} else {
			_ = os.Unsetenv(extracmd.EnvVar)
		}
	}()

	_ = os.Unsetenv(extracmd.EnvVar)
	extracmd.ApplyVisibility()
	require.True(t, c.Hidden)

	_ = os.Setenv(extracmd.EnvVar, "true")
	extracmd.ApplyVisibility()
	require.False(t, c.Hidden)
}
