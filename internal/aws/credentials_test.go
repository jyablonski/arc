package aws

import (
	"os"
	"path/filepath"
	"testing"
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
			if result != tt.expected {
				t.Errorf("MaskKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
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
	if err != nil {
		t.Fatalf("Failed to create test credentials file: %v", err)
	}

	profiles, err := ReadCredentials(credentialsPath)
	if err != nil {
		t.Fatalf("ReadCredentials() error = %v", err)
	}

	// Check default profile
	defaultProfile, exists := profiles["default"]
	if !exists {
		t.Error("default profile not found")
	}
	if defaultProfile["aws_access_key_id"] != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("default aws_access_key_id = %q, want %q", defaultProfile["aws_access_key_id"], "AKIAIOSFODNN7EXAMPLE")
	}
	if defaultProfile["aws_secret_access_key"] != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("default aws_secret_access_key mismatch")
	}

	// Check profile1
	profile1, exists := profiles["profile1"]
	if !exists {
		t.Error("profile1 not found")
	}
	if profile1["aws_access_key_id"] != "AKIAEXAMPLE123" {
		t.Errorf("profile1 aws_access_key_id = %q, want %q", profile1["aws_access_key_id"], "AKIAEXAMPLE123")
	}
	if profile1["region"] != "us-west-2" {
		t.Errorf("profile1 region = %q, want %q", profile1["region"], "us-west-2")
	}

	// Check profile2
	profile2, exists := profiles["profile2"]
	if !exists {
		t.Error("profile2 not found")
	}
	if profile2["aws_access_key_id"] != "AKIAEXAMPLE456" {
		t.Errorf("profile2 aws_access_key_id = %q, want %q", profile2["aws_access_key_id"], "AKIAEXAMPLE456")
	}
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
	if err != nil {
		t.Fatalf("WriteCredentials() error = %v", err)
	}

	// Read it back and verify
	readProfiles, err := ReadCredentials(credentialsPath)
	if err != nil {
		t.Fatalf("ReadCredentials() error = %v", err)
	}

	// Check default profile
	if readProfiles["default"]["aws_access_key_id"] != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("default aws_access_key_id = %q, want %q", readProfiles["default"]["aws_access_key_id"], "AKIAIOSFODNN7EXAMPLE")
	}

	// Check profile1
	if readProfiles["profile1"]["aws_access_key_id"] != "AKIAEXAMPLE123" {
		t.Errorf("profile1 aws_access_key_id = %q, want %q", readProfiles["profile1"]["aws_access_key_id"], "AKIAEXAMPLE123")
	}
	if readProfiles["profile1"]["region"] != "us-west-2" {
		t.Errorf("profile1 region = %q, want %q", readProfiles["profile1"]["region"], "us-west-2")
	}
}

func TestBackupCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")
	backupContent := "test credentials content"

	// Create original file
	err := os.WriteFile(credentialsPath, []byte(backupContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test credentials file: %v", err)
	}

	// Create backup
	backupPath, err := BackupCredentials(credentialsPath)
	if err != nil {
		t.Fatalf("BackupCredentials() error = %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", backupPath)
	}

	// Verify backup content matches original
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if string(backupData) != backupContent {
		t.Errorf("Backup content = %q, want %q", string(backupData), backupContent)
	}

	// Verify backup filename has .bak extension
	if filepath.Ext(backupPath) != ".bak" {
		t.Errorf("Backup file should have .bak extension, got: %s", backupPath)
	}
}

func TestReadCredentialsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsPath := filepath.Join(tmpDir, "credentials")

	// Create empty file
	err := os.WriteFile(credentialsPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test credentials file: %v", err)
	}

	profiles, err := ReadCredentials(credentialsPath)
	if err != nil {
		t.Fatalf("ReadCredentials() error = %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("Expected empty profiles map, got %d profiles", len(profiles))
	}
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
	if err != nil {
		t.Fatalf("Failed to create test credentials file: %v", err)
	}

	profiles, err := ReadCredentials(credentialsPath)
	if err != nil {
		t.Fatalf("ReadCredentials() error = %v", err)
	}

	if profiles["default"]["aws_access_key_id"] != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Failed to parse credentials with comments")
	}
}
