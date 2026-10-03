package cmd

import (
	"os"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "arc",
	Short: "Maintain Arch Linux and macOS machines and local AI-tool workflows",
	Long: `arc is a personal CLI for maintaining Arch Linux and macOS machines and
managing local AI-tool workflows. It wraps native system tools and coordinates
AI-tool configuration and usage with consistent commands, output, and JSON support.`,
	Version: version,
	// Execute prints the error itself, once, in the shared grammar.
	SilenceErrors: true,
	// Usage is for a command that could not be parsed. Once a command is
	// running, a failure is about the work, not about how it was invoked.
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true
	},
}

func Execute() {
	start := time.Now()
	cmd, err := rootCmd.ExecuteC()
	recordInvocation(cmd, err == nil, time.Since(start))
	if err != nil {
		output.Failure(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolP("json", "j", false, "Output in JSON format")
}
