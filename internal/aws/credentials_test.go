package aws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short key",
			input:    "ABC",
			expected: "****",
		},
		{
			name:     "exactly 4 characters",
			input:    "ABCD",
			expected: "****",
		},
		{
			name:     "access key format",
			input:    "AKIAIOSFODNN7EXAMPLE",
			expected: "****************MPLE", // 20 chars - 4 = 16 asterisks
		},
		{
			name:     "long key",
			input:    "AKIAIOSFODNN7EXAMPLE1234567890",
			expected: "**************************7890",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadCredentials(t *testing.T) {
	// Create a temporary credentials file
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")

	content := `[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

[profile1]
aws_access_key_id = AKIAEXAMPLE123
aws_secret_access_key = secretkey123
region = us-west-2

# Comment line
[profile2]
aws_access_key_id = AKIAEXAMPLE456
`

	err := os.WriteFile(credentialsPath, []byte(content), 0644)
	require.NoError(t, err, "Failed to create test credentials file")

	profiles, err := ReadCredentials(credentialsPath)
	require.NoError(t, err)

	// Check default profile
	defaultProfile, exists := profiles["default"]
	require.True(t, exists, "default profile not found")
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", defaultProfile["aws_access_key_id"])
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", defaultProfile["aws_secret_access_key"])

	// Check profile1
	profile1, exists := profiles["profile1"]
	require.True(t, exists, "profile1 not found")
	assert.Equal(t, "AKIAEXAMPLE123", profile1["aws_access_key_id"])
	assert.Equal(t, "us-west-2", profile1["region"])

	// Check profile2
	profile2, exists := profiles["profile2"]
	require.True(t, exists, "profile2 not found")
	assert.Equal(t, "AKIAEXAMPLE456", profile2["aws_access_key_id"])
}

func TestWriteCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")

	profiles := map[string]map[string]string{
		"default": {
			"aws_access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
		"profile1": {
			"aws_access_key_id": "AKIAEXAMPLE123",
			"region":            "us-west-2",
		},
	}

	err := WriteCredentials(credentialsPath, profiles)
	require.NoError(t, err)

	// Read it back and verify
	readProfiles, err := ReadCredentials(credentialsPath)
	require.NoError(t, err)

	// Check default profile
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", readProfiles["default"]["aws_access_key_id"])

	// Check profile1
	assert.Equal(t, "AKIAEXAMPLE123", readProfiles["profile1"]["aws_access_key_id"])
	assert.Equal(t, "us-west-2", readProfiles["profile1"]["region"])
}

func TestBackupCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")
	backupContent := "test credentials content"

	// Create original file
	err := os.WriteFile(credentialsPath, []byte(backupContent), 0644)
	require.NoError(t, err, "Failed to create test credentials file")

	// Create backup
	backupPath, err := BackupCredentials(credentialsPath)
	require.NoError(t, err)

	// Verify backup file exists
	_, err = os.Stat(backupPath)
	assert.False(t, os.IsNotExist(err), "Backup file should exist: %s", backupPath)

	// Verify backup content matches original
	backupData, err := os.ReadFile(backupPath)
	require.NoError(t, err, "Failed to read backup file")
	assert.Equal(t, backupContent, string(backupData))

	// Verify backup filename has .bak extension
	assert.Equal(t, ".bak", filepath.Ext(backupPath))
}

func TestReadCredentialsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")

	// Create empty file
	err := os.WriteFile(credentialsPath, []byte(""), 0644)
	require.NoError(t, err, "Failed to create test credentials file")

	profiles, err := ReadCredentials(credentialsPath)
	require.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestReadCredentialsWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")

	content := `# This is a comment
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
# Another comment
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`

	err := os.WriteFile(credentialsPath, []byte(content), 0644)
	require.NoError(t, err, "Failed to create test credentials file")

	profiles, err := ReadCredentials(credentialsPath)
	require.NoError(t, err)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", profiles["default"]["aws_access_key_id"])
}
