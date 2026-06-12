package aws

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
)

// RotateKeys rotates the IAM access keys for profileName in the credentials
// file at credentialsPath.
//
// The flow is intentionally safe-by-default: it backs up the file, creates a
// new key, writes it, and validates it BEFORE deleting any old keys. If the new
// key fails validation the backup is restored so the existing credentials stay
// active and no keys are deleted. A failure to delete an old key after a
// successful rotation is only a warning — the rotation itself has succeeded.
//
// maxRetries bounds validation polling (the command passes 10; tests pass 1 to
// avoid the multi-second sleep between attempts).
func RotateKeys(credentialsPath, profileName string, maxRetries int) error {
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return fmt.Errorf("AWS credentials file not found at %s", credentialsPath)
	}

	output.Info("Backing up existing credentials...")
	backupPath, err := BackupCredentials(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to backup credentials: %w", err)
	}
	output.Success(fmt.Sprintf("Credentials backed up to: %s", backupPath))

	profiles, err := ReadCredentials(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	profile, exists := profiles[profileName]
	if !exists {
		return fmt.Errorf("profile '%s' not found in credentials file", profileName)
	}

	oldAccessKey := profile["aws_access_key_id"]
	if oldAccessKey == "" {
		return fmt.Errorf("no access key found in profile '%s'", profileName)
	}

	output.Info(fmt.Sprintf("Using profile: %s", profileName))
	output.Info(fmt.Sprintf("Current access key: %s", MaskKey(oldAccessKey)))

	identity, err := GetCurrentIdentity()
	if err != nil {
		return fmt.Errorf("failed to get current identity: %w", err)
	}

	userArn, ok := identity["Arn"].(string)
	if !ok {
		return arcerrs.ErrNoUserARN
	}

	username, err := GetUsername(userArn)
	if err != nil {
		return err
	}

	output.Info(fmt.Sprintf("Rotating keys for user: %s", username))

	oldKeyIDs, err := ListAccessKeys(username)
	if err != nil {
		return err
	}

	if len(oldKeyIDs) == 0 {
		output.Info("No access keys found")
		return nil
	}

	output.Info("Creating new access key...")
	newKeyID, newSecretKey, err := CreateAccessKey(username)
	if err != nil {
		return err
	}
	output.Success(fmt.Sprintf("Created new access key: %s", MaskKey(newKeyID)))

	output.Info("Updating credentials file...")
	profile["aws_access_key_id"] = newKeyID
	profile["aws_secret_access_key"] = newSecretKey
	profiles[profileName] = profile

	if err := WriteCredentials(credentialsPath, profiles); err != nil {
		return fmt.Errorf("failed to update credentials file: %w", err)
	}
	output.Success("Credentials file updated")

	output.Info("Testing new credentials (may take a few seconds to propagate)...")
	testOutput, err := ValidateCredentials(newKeyID, newSecretKey, maxRetries)
	if err != nil {
		output.Error(fmt.Sprintf("New credentials failed validation: %s", err))
		output.Info("Restoring backup...")
		if restoreErr := run.RunInteractive("cp", backupPath, credentialsPath); restoreErr != nil {
			return fmt.Errorf("failed to restore backup: %w (original error: %s)", restoreErr, err)
		}
		output.Success("Backup restored - old credentials are still active")
		output.Warning("The new access key was created but failed validation.")
		output.Warning(fmt.Sprintf("New access key ID: %s", MaskKey(newKeyID)))
		output.Warning("You may need to wait a few minutes and manually update ~/.aws/credentials")
		output.Warning(fmt.Sprintf("Backup saved at: %s", backupPath))
		return fmt.Errorf("new credentials failed validation: %s", err)
	}

	// Best-effort confirmation that the new key resolves to the same identity.
	var testIdentity map[string]interface{}
	if err := json.Unmarshal([]byte(testOutput), &testIdentity); err == nil {
		testArn, _ := testIdentity["Arn"].(string)
		if testArn == userArn {
			output.Success("New credentials validated successfully")
		}
	}

	output.Info("Deleting old access keys...")
	for _, keyID := range oldKeyIDs {
		output.Info(fmt.Sprintf("Deleting old access key: %s", MaskKey(keyID)))
		if err := DeleteAccessKey(username, keyID); err != nil {
			output.Warning(fmt.Sprintf("Failed to delete key %s: %v", MaskKey(keyID), err))
			output.Warning("You may need to delete this key manually from the AWS console")
		}
	}

	output.Info("Cleaning up backup file...")
	if err := os.Remove(backupPath); err != nil {
		output.Warning(fmt.Sprintf("Failed to remove backup file %s: %v", backupPath, err))
	} else {
		output.Success("Backup file removed")
	}

	output.Success("Key rotation complete!")
	output.Info(fmt.Sprintf("New access key: %s", MaskKey(newKeyID)))

	return nil
}
