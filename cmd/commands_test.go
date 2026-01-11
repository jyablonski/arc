package cmd

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	"info",
	"installed",
	"packages",
	"parts",
	"setup",
	"sleep",
	"update",
	"validate",
	// AWS subcommands
	"aws rotate-keys",
	"aws whoami",
	// GH subcommands
	"gh restart-dashboard",
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

func TestAllCommandsMatchExpectedList(t *testing.T) {
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
		t.Error("Command list mismatch detected!")

		if len(missing) > 0 {
			t.Errorf("\nMissing from expectedCommands (add these):")
			for _, cmd := range missing {
				t.Errorf("  %q,", cmd)
			}
		}

		if len(extra) > 0 {
			t.Errorf("\nExtra in expectedCommands (remove these):")
			for _, cmd := range extra {
				t.Errorf("  %q,", cmd)
			}
		}

		t.Errorf("\nActual commands found:")
		for _, cmd := range allCommands {
			t.Errorf("  %q,", cmd)
		}
	}
}

func TestExpectedCommandsAreValid(t *testing.T) {
	// Verify that all expected commands actually exist
	allCommands := getAllCommands(rootCmd, "")
	actualMap := make(map[string]bool)
	for _, cmd := range allCommands {
		actualMap[cmd] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !actualMap[expectedCmd] {
			t.Errorf("Expected command %q not found in actual commands", expectedCmd)
		}
	}
}

func TestCommandPathsAreWellFormed(t *testing.T) {
	allCommands := getAllCommands(rootCmd, "")

	for _, cmd := range allCommands {
		// Commands should not be empty
		if strings.TrimSpace(cmd) == "" {
			t.Errorf("Found empty command path")
		}

		// Commands should not have leading/trailing spaces
		if cmd != strings.TrimSpace(cmd) {
			t.Errorf("Command %q has leading/trailing spaces", cmd)
		}

		// Commands should not have double spaces
		if strings.Contains(cmd, "  ") {
			t.Errorf("Command %q contains double spaces", cmd)
		}
	}
}
