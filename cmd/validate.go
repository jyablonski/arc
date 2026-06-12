package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/deps"
	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate that all required tools are available",
	Long: `Check if all required and optional tools are available in PATH.
Required tools are necessary for basic functionality, while optional tools
enable additional features.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Header("Validating arc dependencies")

		output.Info(fmt.Sprintf("Platform: %s", app.Platform))

		tools := append([]deps.ToolStatus(nil), app.Tools...)

		// Check availability
		for i := range tools {
			tools[i].Available = run.CommandExists(tools[i].Name)
		}

		// Separate required and optional
		required := []deps.ToolStatus{}
		optional := []deps.ToolStatus{}
		for _, tool := range tools {
			if tool.Required {
				required = append(required, tool)
			} else {
				optional = append(optional, tool)
			}
		}

		// Check required tools
		output.Header("Required Tools")
		allRequiredAvailable := true
		for _, tool := range required {
			if tool.Available {
				output.Success(fmt.Sprintf("✓ %s - %s", tool.Name, tool.Description))
			} else {
				output.Error(fmt.Sprintf("✗ %s - %s (MISSING)", tool.Name, tool.Description))
				allRequiredAvailable = false
			}
		}

		// Check optional tools
		output.Header("Optional Tools")
		for _, tool := range optional {
			if tool.Available {
				output.Success(fmt.Sprintf("✓ %s - %s", tool.Name, tool.Description))
			} else {
				output.Info(fmt.Sprintf("○ %s - %s (not installed)", tool.Name, tool.Description))
			}
		}

		// Summary
		fmt.Println()
		if allRequiredAvailable {
			output.Success("All required tools are available!")
			return nil
		} else {
			output.Error("Some required tools are missing!")
			output.Info("Run 'arc setup' to install missing dependencies")
			return arcerrs.ErrValidationFailed
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
