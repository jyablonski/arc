package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/shell"
)

// GetCurrentIdentity returns the current AWS identity
func GetCurrentIdentity() (map[string]interface{}, error) {
	output, err := shell.Run("aws", "sts", "get-caller-identity")
	if err != nil {
		return nil, fmt.Errorf("failed to get current identity: %w", err)
	}

	var identity map[string]interface{}
	if err := json.Unmarshal([]byte(output), &identity); err != nil {
		return nil, fmt.Errorf("failed to parse identity: %w", err)
	}

	return identity, nil
}

// GetUsername extracts username from ARN
func GetUsername(arn string) (string, error) {
	parts := strings.Split(arn, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid ARN format")
	}
	return parts[len(parts)-1], nil
}

// ListAccessKeys lists all access keys for a user
func ListAccessKeys(username string) ([]string, error) {
	output, err := shell.Run("aws", "iam", "list-access-keys", "--user-name", username)
	if err != nil {
		return nil, fmt.Errorf("failed to list access keys: %w", err)
	}

	var keysResponse map[string]interface{}
	if err := json.Unmarshal([]byte(output), &keysResponse); err != nil {
		return nil, fmt.Errorf("failed to parse keys list: %w", err)
	}

	keys, ok := keysResponse["AccessKeyMetadata"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse access keys")
	}

	keyIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		keyMap := key.(map[string]interface{})
		keyID := keyMap["AccessKeyId"].(string)
		keyIDs = append(keyIDs, keyID)
	}

	return keyIDs, nil
}

// CreateAccessKey creates a new access key for a user
func CreateAccessKey(username string) (string, string, error) {
	output, err := shell.Run("aws", "iam", "create-access-key", "--user-name", username)
	if err != nil {
		return "", "", fmt.Errorf("failed to create new access key: %w", err)
	}

	var newKey map[string]interface{}
	if err := json.Unmarshal([]byte(output), &newKey); err != nil {
		return "", "", fmt.Errorf("failed to parse new key: %w", err)
	}

	accessKey := newKey["AccessKey"].(map[string]interface{})
	keyID := accessKey["AccessKeyId"].(string)
	secretKey := accessKey["SecretAccessKey"].(string)

	return keyID, secretKey, nil
}

// DeleteAccessKey deletes an access key
func DeleteAccessKey(username, keyID string) error {
	_, err := shell.Run("aws", "iam", "delete-access-key", "--user-name", username, "--access-key-id", keyID)
	return err
}

// ValidateCredentials validates credentials by testing them with sts get-caller-identity
func ValidateCredentials(accessKeyID, secretAccessKey string, maxRetries int) (string, error) {
	// Prepare environment with new credentials
	testEnv := os.Environ()
	// Remove existing AWS credential env vars
	filteredEnv := make([]string, 0, len(testEnv))
	for _, env := range testEnv {
		if !strings.HasPrefix(env, "AWS_ACCESS_KEY_ID=") &&
			!strings.HasPrefix(env, "AWS_SECRET_ACCESS_KEY=") &&
			!strings.HasPrefix(env, "AWS_SESSION_TOKEN=") {
			filteredEnv = append(filteredEnv, env)
		}
	}
	// Add new credentials as env vars
	filteredEnv = append(filteredEnv,
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretAccessKey),
	)

	// Retry validation with delays (AWS credentials may need time to propagate)
	var testOutput string
	var validationErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			time.Sleep(4 * time.Second)
		}

		testCmd := exec.Command("aws", "sts", "get-caller-identity")
		testCmd.Env = filteredEnv

		// Capture both stdout and stderr for better error messages
		var stdout, stderr bytes.Buffer
		testCmd.Stdout = &stdout
		testCmd.Stderr = &stderr

		validationErr = testCmd.Run()
		if validationErr == nil {
			testOutput = stdout.String()
			return testOutput, nil
		}
	}

	// Get error message from last attempt
	testCmd := exec.Command("aws", "sts", "get-caller-identity")
	testCmd.Env = filteredEnv
	var stderr bytes.Buffer
	testCmd.Stderr = &stderr
	testCmd.Run() // Run again just to capture stderr
	errorMsg := stderr.String()
	if errorMsg == "" {
		errorMsg = validationErr.Error()
	}

	return "", fmt.Errorf("validation failed after %d attempts: %s", maxRetries, errorMsg)
}
