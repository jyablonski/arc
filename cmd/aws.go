package cmd

import (
	"encoding/json"
	"os"

	"github.com/jyablonski/arc/internal/aws"
	"github.com/jyablonski/arc/internal/output"
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

var awsRotateKeysCmd = &cobra.Command{
	Use:   "rotate-keys",
	Short: "Rotate IAM access keys",
	Long: `Rotate IAM access keys by creating new keys and deleting old ones.
Automatically backs up old credentials and updates ~/.aws/credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !run.CommandExists("aws") {
			return shell.NewErrToolNotAvailable("aws")
		}

		output.Header("Rotating IAM Access Keys")

		credentialsPath, err := aws.GetCredentialsPath()
		if err != nil {
			return err
		}

		// Determine which profile to use (default or from AWS_PROFILE env).
		profileName := "default"
		if envProfile := os.Getenv("AWS_PROFILE"); envProfile != "" {
			profileName = envProfile
		}

		return aws.RotateKeys(credentialsPath, profileName, 10)
	},
}

func init() {
	rootCmd.AddCommand(awsCmd)
	awsCmd.AddCommand(awsWhoamiCmd)
	awsCmd.AddCommand(awsRotateKeysCmd)
}
