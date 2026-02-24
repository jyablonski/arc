package cmd

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedCommands is the canonical list of all available commands.
// Update this list when adding new commands or subcommands.
var expectedCommands = []string{
	// Root-level commands
	"aws",
	"clean",
	"docker clean",
	"gh",
	"git cleanup",
	"incident [title]",
	"info",
	"installed",
	"packages",
	"parts",
	"self",
	"setup",
	"sleep",
	"update",
	"validate",
	// AWS subcommands
	"aws rotate-keys",
	"aws whoami",
	// GH subcommands
	"gh restart-dashboard",
	// Self subcommands
	"self update",
	// Update subcommands
	"update uv",
}

// getAllCommands recursively collects all command paths from the root command
func getAllCommands(cmd *cobra.Command, prefix string) []string {
	var commands []string

	// Skip the root command itself ("arc")
	if cmd.Use == "arc" {
		prefix = ""
	} else {
		// Build the full command path
		if prefix != "" {
			commands = append(commands, prefix+" "+cmd.Use)
			prefix = prefix + " " + cmd.Use
		} else {
			commands = append(commands, cmd.Use)
			prefix = cmd.Use
		}
	}

	// Recursively collect subcommands
	for _, subcmd := range cmd.Commands() {
		// Skip help and completion commands added by cobra
		if subcmd.Use == "help" || subcmd.Use == "completion" {
			continue
		}
		commands = append(commands, getAllCommands(subcmd, prefix)...)
	}

	return commands
}

func TestCommands(t *testing.T) {
	t.Run("When comparing actual commands to expected list, they match", func(t *testing.T) {
		// Get all commands from the root command tree
		allCommands := getAllCommands(rootCmd, "")

		// Sort for consistent comparison
		sort.Strings(allCommands)

		// Create a map for easier lookup
		expectedMap := make(map[string]bool)
		for _, cmd := range expectedCommands {
			expectedMap[cmd] = true
		}

		actualMap := make(map[string]bool)
		for _, cmd := range allCommands {
			actualMap[cmd] = true
		}

		// Find missing commands (in actual but not in expected)
		var missing []string
		for _, cmd := range allCommands {
			if !expectedMap[cmd] {
				missing = append(missing, cmd)
			}
		}

		// Find extra commands (in expected but not in actual)
		var extra []string
		for _, cmd := range expectedCommands {
			if !actualMap[cmd] {
				extra = append(extra, cmd)
			}
		}

		// Report any discrepancies
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
