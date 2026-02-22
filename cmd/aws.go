package cmd

import (
	"encoding/json"
	"fmt"
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
		if !shell.CommandExists("aws") {
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
		if !shell.CommandExists("aws") {
			return shell.NewErrToolNotAvailable("aws")
		}

		output.Header("Rotating IAM Access Keys")

		// Get AWS credentials file path
		credentialsPath, err := aws.GetCredentialsPath()
		if err != nil {
			return err
		}

		// Check if credentials file exists
		if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
			return fmt.Errorf("AWS credentials file not found at %s", credentialsPath)
		}

		// Backup credentials file
		output.Info("Backing up existing credentials...")
		backupPath, err := aws.BackupCredentials(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to backup credentials: %w", err)
		}
		output.Success(fmt.Sprintf("Credentials backed up to: %s", backupPath))

		// Read current credentials
		profiles, err := aws.ReadCredentials(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to read credentials file: %w", err)
		}

		// Determine which profile to use (default or from AWS_PROFILE env)
		profileName := "default"
		if envProfile := os.Getenv("AWS_PROFILE"); envProfile != "" {
			profileName = envProfile
		}

		// Check if profile exists
		profile, exists := profiles[profileName]
		if !exists {
			return fmt.Errorf("profile '%s' not found in credentials file", profileName)
		}

		oldAccessKey := profile["aws_access_key_id"]
		if oldAccessKey == "" {
			return fmt.Errorf("no access key found in profile '%s'", profileName)
		}

		output.Info(fmt.Sprintf("Using profile: %s", profileName))
		output.Info(fmt.Sprintf("Current access key: %s", aws.MaskKey(oldAccessKey)))

		// Get current user from AWS
		identity, err := aws.GetCurrentIdentity()
		if err != nil {
			return fmt.Errorf("failed to get current identity: %w", err)
		}

		userArn, ok := identity["Arn"].(string)
		if !ok {
			return ErrNoUserARN
		}

		username, err := aws.GetUsername(userArn)
		if err != nil {
			return err
		}

		output.Info(fmt.Sprintf("Rotating keys for user: %s", username))

		// List existing keys
		oldKeyIDs, err := aws.ListAccessKeys(username)
		if err != nil {
			return err
		}

		if len(oldKeyIDs) == 0 {
			output.Info("No access keys found")
			return nil
		}

		// Create new key
		output.Info("Creating new access key...")
		newKeyID, newSecretKey, err := aws.CreateAccessKey(username)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created new access key: %s", aws.MaskKey(newKeyID)))

		// Update credentials file with new keys
		output.Info("Updating credentials file...")
		profile["aws_access_key_id"] = newKeyID
		profile["aws_secret_access_key"] = newSecretKey
		profiles[profileName] = profile

		if err := aws.WriteCredentials(credentialsPath, profiles); err != nil {
			return fmt.Errorf("failed to update credentials file: %w", err)
		}
		output.Success("Credentials file updated")

		// Test new credentials work
		output.Info("Testing new credentials (may take a few seconds to propagate)...")
		testOutput, err := aws.ValidateCredentials(newKeyID, newSecretKey, 10)
		if err != nil {
			output.Error(fmt.Sprintf("New credentials failed validation: %s", err))
			output.Info("Restoring backup...")
			// Restore backup
			if restoreErr := shell.RunInteractive("cp", backupPath, credentialsPath); restoreErr != nil {
				return fmt.Errorf("failed to restore backup: %w (original error: %s)", restoreErr, err)
			}
			output.Success("Backup restored - old credentials are still active")
			output.Warning("The new access key was created but failed validation.")
			output.Warning(fmt.Sprintf("New access key ID: %s", aws.MaskKey(newKeyID)))
			output.Warning("You may need to wait a few minutes and manually update ~/.aws/credentials")
			output.Warning(fmt.Sprintf("Backup saved at: %s", backupPath))
			return fmt.Errorf("new credentials failed validation: %s", err)
		}

		// Verify the identity matches
		var testIdentity map[string]interface{}
		if err := json.Unmarshal([]byte(testOutput), &testIdentity); err == nil {
			testArn, _ := testIdentity["Arn"].(string)
			if testArn == userArn {
				output.Success("New credentials validated successfully")
			}
		}

		// Delete old keys
		output.Info("Deleting old access keys...")
		for _, keyID := range oldKeyIDs {
			output.Info(fmt.Sprintf("Deleting old access key: %s", aws.MaskKey(keyID)))
			if err := aws.DeleteAccessKey(username, keyID); err != nil {
				output.Warning(fmt.Sprintf("Failed to delete key %s: %v", aws.MaskKey(keyID), err))
				output.Warning("You may need to delete this key manually from the AWS console")
			}
		}

		// Clean up backup file after successful rotation
		output.Info("Cleaning up backup file...")
		if err := os.Remove(backupPath); err != nil {
			output.Warning(fmt.Sprintf("Failed to remove backup file %s: %v", backupPath, err))
		} else {
			output.Success("Backup file removed")
		}

		output.Success("Key rotation complete!")
		output.Info(fmt.Sprintf("New access key: %s", aws.MaskKey(newKeyID)))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(awsCmd)
	awsCmd.AddCommand(awsWhoamiCmd)
	awsCmd.AddCommand(awsRotateKeysCmd)
}
