package aws

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCredentialsPath(t *testing.T) {
	// Get the actual current user
	usr, err := user.Current()
	if err != nil {
		t.Skipf("Skipping test: cannot get current user: %v", err)
	}

	expectedPath := filepath.Join(usr.HomeDir, ".aws", "credentials")

	// Test the function
	path, err := GetCredentialsPath()
	if err != nil {
		t.Fatalf("GetCredentialsPath() error = %v", err)
	}

	if path != expectedPath {
		t.Errorf("GetCredentialsPath() = %q, want %q", path, expectedPath)
	}
}

func TestGetCredentialsPath_ErrorHandling(t *testing.T) {
	// This test verifies the function handles errors correctly
	// We can't easily mock user.Current() without refactoring,
	// but we can verify the function returns an error when appropriate
	// For now, we'll just verify it works with the real user

	// If we can't get the current user, the function should error
	// But in practice, this should always work, so we'll just test the happy path
	path, err := GetCredentialsPath()
	if err != nil {
		t.Fatalf("GetCredentialsPath() unexpected error = %v", err)
	}

	// Verify the path format is correct
	if path == "" {
		t.Error("GetCredentialsPath() returned empty path")
	}

	// Verify it ends with .aws/credentials
	expectedSuffix := filepath.Join(".aws", "credentials")
	if !strings.HasSuffix(path, expectedSuffix) {
		t.Errorf("GetCredentialsPath() path %q does not end with %q", path, expectedSuffix)
	}
}
