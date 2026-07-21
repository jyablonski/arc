package cmd

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expectedCommands = []string{
	"ai",
	"ai health",
	"ai pricing",
	"ai sessions",
	"ai tokens",
	"ai usage",
	"aws",
	"help [command]",
	"clean",
	"docker clean",
	"git cleanup",
	"incident [title]",
	"info",
	"installed",
	"packages",
	"parts",
	"setup",
	"sleep",
	"stats",
	"update",
	"update self",
	"update system",
	"validate",
	"aws whoami",
	"update uv",
	"skills",
	"skills add [path]",
	"skills sync",
	"skills export <parent_folder>",
	"skills list",
	"skills validate [name]",
	"skills remove <name>",
	"skills prune",
	"rules",
	"rules sync",
	"rules status",
}

func getAllCommands(cmd *cobra.Command, prefix string) []string {
	var commands []string

	if cmd.Use == "arc" {
		prefix = ""
	} else {
		if prefix != "" {
			commands = append(commands, prefix+" "+cmd.Use)
			prefix = prefix + " " + cmd.Use
		} else {
			commands = append(commands, cmd.Use)
			prefix = cmd.Use
		}
	}

	for _, subcmd := range cmd.Commands() {
		if subcmd.Use == "help" || subcmd.Use == "completion" || subcmd.Hidden {
			continue
		}
		commands = append(commands, getAllCommands(subcmd, prefix)...)
	}

	return commands
}

func TestCommands(t *testing.T) {
	t.Run("When comparing actual commands to expected list, they match", func(t *testing.T) {
		allCommands := getAllCommands(rootCmd, "")

		sort.Strings(allCommands)

		expectedMap := make(map[string]bool)
		for _, cmd := range expectedCommands {
			expectedMap[cmd] = true
		}

		actualMap := make(map[string]bool)
		for _, cmd := range allCommands {
			actualMap[cmd] = true
		}

		var missing []string
		for _, cmd := range allCommands {
			if !expectedMap[cmd] {
				missing = append(missing, cmd)
			}
		}

		var extra []string
		for _, cmd := range expectedCommands {
			if !actualMap[cmd] {
				extra = append(extra, cmd)
			}
		}

		if len(missing) > 0 || len(extra) > 0 {
			if len(missing) > 0 {
				t.Errorf("Missing from expectedCommands (add these): %v", missing)
			}
			if len(extra) > 0 {
				t.Errorf("Extra in expectedCommands (remove these): %v", extra)
			}
			t.Errorf("Actual commands found: %v", allCommands)
		}
	})

	t.Run("When checking expected commands exist, they are all found", func(t *testing.T) {
		allCommands := getAllCommands(rootCmd, "")
		actualMap := make(map[string]bool)
		for _, cmd := range allCommands {
			actualMap[cmd] = true
		}

		for _, expectedCmd := range expectedCommands {
			assert.True(t, actualMap[expectedCmd], "Expected command %q not found in actual commands", expectedCmd)
		}
	})

	t.Run("When checking command paths, they are well-formed", func(t *testing.T) {
		allCommands := getAllCommands(rootCmd, "")

		for _, cmd := range allCommands {
			require.NotEmpty(t, strings.TrimSpace(cmd), "Found empty command path")
			assert.Equal(t, strings.TrimSpace(cmd), cmd, "Command %q has leading/trailing spaces", cmd)
			assert.False(t, strings.Contains(cmd, "  "), "Command %q contains double spaces", cmd)
		}
	})

}
