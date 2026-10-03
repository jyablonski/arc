package cmd

import (
	"fmt"
	"strings"

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

		output.Title("arc docker clean", "")
		prunes := []struct {
			kind string
			args []string
		}{
			{"images", []string{"image", "prune", "-af"}},
			{"containers", []string{"container", "prune", "-f"}},
			{"volumes", []string{"volume", "prune", "-f"}},
		}
		// A failed prune does not stop the others or fail the command.
		failed := 0
		for _, p := range prunes {
			out, err := run.Run("docker", p.args...)
			if err != nil {
				failed++
				output.Warning(fmt.Sprintf("%s not pruned: %v", p.kind, err))
				continue
			}
			output.Success(p.kind + " pruned" + dockerReclaimed(out))
		}

		if failed > 0 {
			output.Summary(output.GlyphWarn, fmt.Sprintf("%d of %d prunes failed", failed, len(prunes)))
			return nil
		}
		output.Summary(output.GlyphOK, "docker cleanup complete")
		return nil
	},
}

// dockerReclaimed pulls the freed size out of a prune's output, e.g.
// "Total reclaimed space: 1.2GB" becomes " (1.2GB)".
func dockerReclaimed(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if size, ok := strings.CutPrefix(strings.TrimSpace(line), "Total reclaimed space:"); ok {
			return " (" + strings.TrimSpace(size) + ")"
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}
