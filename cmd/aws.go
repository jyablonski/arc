package cmd

import (
	"encoding/json"
	"os"

	"github.com/jyablonski/arc/internal/aws"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var awsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS services.`,
}

var awsWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current AWS identity",
	Long:  `Show the current AWS identity using sts get-caller-identity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !run.CommandExists("aws") {
			return shell.NewErrToolNotAvailable("aws")
		}

		identity, err := aws.GetCurrentIdentity()
		if err != nil {
			return err
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(identity)
	},
}

func init() {
	rootCmd.AddCommand(awsCmd)
	awsCmd.AddCommand(awsWhoamiCmd)
}
