package cmd

import (
	"fmt"
	"os"
	"time"

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
}

func Execute() {
	start := time.Now()
	cmd, err := rootCmd.ExecuteC()
	recordInvocation(cmd, err == nil, time.Since(start))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolP("json", "j", false, "Output in JSON format")
}
