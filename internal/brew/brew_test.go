package brew

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestListFormulae(t *testing.T) {
	shell.SetMockRunner(&shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "brew", name)
			require.Equal(t, []string{"list", "--formula"}, args)
			return "git\nuv\nfastfetch\n", nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	got, err := ListFormulae()
	require.NoError(t, err)
	require.Equal(t, []string{"git", "uv", "fastfetch"}, got)
}

func TestInstalledInfo(t *testing.T) {
	shell.SetMockRunner(&shell.MockRunner{
		RunFunc: func(name string, args ...string) (string, error) {
			require.Equal(t, "brew", name)
			require.Equal(t, []string{"info", "--json=v2", "--installed"}, args)
			return `{"formulae":[{"name":"git"}],"casks":[{"name":"cursor"}]}`, nil
		},
	})
	t.Cleanup(shell.ClearMockRunner)

	got, err := InstalledInfo()
	require.NoError(t, err)
	require.Len(t, got.Formulae, 1)
	require.Equal(t, "git", got.Formulae[0].Name)
	require.Len(t, got.Casks, 1)
	require.Equal(t, "cursor", got.Casks[0].Name)
}

func TestCacheSize(t *testing.T) {
	shell.SetMockRunner(&shell.MockRunner{
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
	t.Cleanup(shell.ClearMockRunner)

	got, err := CacheSize()
	require.NoError(t, err)
	require.Equal(t, "1.2G", got)
}
