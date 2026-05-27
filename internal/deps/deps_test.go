package deps

import (
	"testing"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestTools(t *testing.T) {
	linux := Tools(platform.Linux)
	require.Contains(t, toolNames(linux), "pacman")
	require.Contains(t, toolNames(linux), "systemctl")

	darwin := Tools(platform.Darwin)
	require.Contains(t, toolNames(darwin), "brew")
	require.Contains(t, toolNames(darwin), "pmset")

	unknown := Tools(platform.Unknown)
	names := toolNames(unknown)
	require.Contains(t, names, "git")
	require.NotContains(t, names, "pacman")
	require.NotContains(t, names, "brew")
}

func toolNames(tools []ToolStatus) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}
