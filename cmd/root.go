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
	Short: "A personal CLI tool for system management",
	Long: `arc is a personal CLI tool for system management, maintenance, and dev workflows
on Linux and macOS. It provides a consistent interface for system operations with
better argument handling, help text, colored output, and error handling.`,
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
