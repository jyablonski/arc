package aws

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// MaskKey masks all but the last 4 characters of a key
func MaskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// GetCredentialsPath returns the path to the AWS credentials file
func GetCredentialsPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".aws", "credentials"), nil
}

// ReadCredentials reads and parses the AWS credentials file
func ReadCredentials(credentialsPath string) (map[string]map[string]string, error) {
	file, err := os.Open(credentialsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	profiles := make(map[string]map[string]string)
	var currentProfile string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for profile section [profile name] or [default]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentProfile = strings.Trim(line, "[]")
			currentProfile = strings.TrimPrefix(currentProfile, "profile ")
			if profiles[currentProfile] == nil {
				profiles[currentProfile] = make(map[string]string)
			}
			continue
		}

		// Parse key=value pairs
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if currentProfile != "" {
				profiles[currentProfile][key] = value
			}
		}
	}

	return profiles, scanner.Err()
}

// WriteCredentials writes profiles back to the credentials file
func WriteCredentials(credentialsPath string, profiles map[string]map[string]string) error {
	file, err := os.Create(credentialsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write default profile first if it exists
	if defaultProfile, exists := profiles["default"]; exists {
		fmt.Fprintln(writer, "[default]")
		for key, value := range defaultProfile {
			fmt.Fprintf(writer, "%s = %s\n", key, value)
		}
		fmt.Fprintln(writer)
		delete(profiles, "default")
	}

	// Write other profiles
	for profileName, profile := range profiles {
		if profileName == "default" {
			continue
		}
		fmt.Fprintf(writer, "[%s]\n", profileName)
		for key, value := range profile {
			fmt.Fprintf(writer, "%s = %s\n", key, value)
		}
		fmt.Fprintln(writer)
	}

	return nil
}

// BackupCredentials creates a backup of the credentials file
func BackupCredentials(credentialsPath string) (string, error) {
	backupPath := credentialsPath + "." + time.Now().Format("20060102-150405") + ".bak"

	sourceFile, err := os.Open(credentialsPath)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(backupPath)
	if err != nil {
		return "", err
	}
	defer backupFile.Close()

	_, err = backupFile.ReadFrom(sourceFile)
	if err != nil {
		return "", err
	}

	return backupPath, nil
}
