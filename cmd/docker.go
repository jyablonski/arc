package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker clean",
	Short: "Clean Docker resources (images, containers, volumes)",
	Long:  `Prune Docker images, containers, and volumes to free up disk space.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !run.CommandExists("docker") {
			return shell.NewErrToolNotAvailable("docker")
		}

		output.Header("Cleaning Docker resources")

		output.Info("Pruning unused images...")
		if _, err := run.Run("docker", "image", "prune", "-af"); err != nil {
			output.Warning(fmt.Sprintf("Failed to prune images: %v", err))
		}

		output.Info("Pruning unused containers...")
		if _, err := run.Run("docker", "container", "prune", "-f"); err != nil {
			output.Warning(fmt.Sprintf("Failed to prune containers: %v", err))
		}

		output.Info("Pruning unused volumes...")
		if _, err := run.Run("docker", "volume", "prune", "-f"); err != nil {
			output.Warning(fmt.Sprintf("Failed to prune volumes: %v", err))
		}

		output.Success("Docker cleanup complete")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}
