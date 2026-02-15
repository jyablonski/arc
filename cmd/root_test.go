package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmdInitialization(t *testing.T) {
	require.NotNil(t, rootCmd, "rootCmd should be initialized")

	assert.Equal(t, "arc", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short, "rootCmd.Short should not be empty")
	assert.NotEmpty(t, rootCmd.Long, "rootCmd.Long should not be empty")
}

func TestRootCmdVersion(t *testing.T) {
	require.NotEmpty(t, version, "version should be set")

	// Version should be "dev" by default or a semantic version
	if version != "dev" {
		assert.True(t, isValidVersion(version), "version %q should be valid format", version)
	}
}

func isValidVersion(v string) bool {
	// Simple check: should start with 'v' followed by numbers and dots
	if len(v) == 0 || v[0] != 'v' {
		return false
	}
	// Basic validation: v followed by at least one digit
	hasDigit := false
	for _, r := range v[1:] {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	return hasDigit
}
