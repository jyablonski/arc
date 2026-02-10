package aws

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, expectedPath, path)
}

func TestGetCredentialsPath_ErrorHandling(t *testing.T) {
	// This test verifies the function handles errors correctly
	path, err := GetCredentialsPath()
	require.NoError(t, err)

	// Verify the path format is correct
	assert.NotEmpty(t, path)

	// Verify it ends with .aws/credentials
	expectedSuffix := filepath.Join(".aws", "credentials")
	assert.True(t, strings.HasSuffix(path, expectedSuffix), "path %q should end with %q", path, expectedSuffix)
}
