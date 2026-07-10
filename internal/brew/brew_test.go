package brew

import (
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/require"
)

func TestListFormulae(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "brew", name)
			require.Equal(t, []string{"list", "--formula"}, args)
			return "git\nuv\nfastfetch\n", nil
		},
	})

	got, err := ListFormulae()
	require.NoError(t, err)
	require.Equal(t, []string{"git", "uv", "fastfetch"}, got)
}

func TestCacheSize(t *testing.T) {
	setRunner(t, &boundary.ShellRunnerMock{
		RunFunc: func(name string, args ...string) (string, error) {
			if name == "brew" {
				require.Equal(t, []string{"--cache"}, args)
				return "/Users/jacob/Library/Caches/Homebrew", nil
			}
			require.Equal(t, "du", name)
			require.Equal(t, []string{"-sh", "/Users/jacob/Library/Caches/Homebrew"}, args)
			return "1.2G\t/Users/jacob/Library/Caches/Homebrew", nil
		},
	})

	got, err := CacheSize()
	require.NoError(t, err)
	require.Equal(t, "1.2G", got)
}
