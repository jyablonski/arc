package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

type ToolStatus struct {
	Name        string
	Required    bool
	Available   bool
	Description string
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate that all required tools are available",
	Long: `Check if all required and optional tools are available in PATH.
Required tools are necessary for basic functionality, while optional tools
enable additional features.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Header("Validating arc dependencies")

		tools := []ToolStatus{
			// Required tools
			{Name: "pacman", Required: true, Description: "Package manager (base system)"},
			{Name: "systemctl", Required: true, Description: "Systemd control (base system)"},
			{Name: "lspci", Required: true, Description: "PCI device lister (pciutils, base system)"},
			{Name: "dmidecode", Required: true, Description: "DMI decode utility (for hardware info)"},
			{Name: "lshw", Required: true, Description: "Hardware lister (for RAM info)"},
			{Name: "git", Required: true, Description: "Git version control"},
			{Name: "gh", Required: true, Description: "GitHub CLI"},
			{Name: "uv", Required: true, Description: "Python package manager"},

			// Optional tools
			{Name: "yay", Required: false, Description: "AUR helper (for AUR updates)"},
			{Name: "docker", Required: false, Description: "Docker (for docker clean command)"},
			{Name: "aws", Required: false, Description: "AWS CLI (for AWS commands)"},
			{Name: "nvidia-smi", Required: false, Description: "NVIDIA driver (for GPU info)"},
			{Name: "paccache", Required: false, Description: "Package cache cleaner"},
			{Name: "gnome-shell", Required: false, Description: "GNOME shell (for system info)"},
		}

		// Check availability
		for i := range tools {
			tools[i].Available = shell.CommandExists(tools[i].Name)
		}

		// Separate required and optional
		required := []ToolStatus{}
		optional := []ToolStatus{}
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
			return fmt.Errorf("validation failed: missing required tools")
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
