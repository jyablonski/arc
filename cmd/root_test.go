package cmd

import (
	"testing"
)

func TestExecute(t *testing.T) {
	// Execute is the main entry point that calls rootCmd.Execute()
	// We can't easily test Execute without it actually running and potentially exiting,
	// so we test that the rootCmd is properly initialized instead
	// The actual execution is tested through integration tests or manual testing
}

func TestRootCmdInitialization(t *testing.T) {
	// Test that rootCmd is properly initialized
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "arc" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "arc")
	}

	if rootCmd.Short == "" {
		t.Error("rootCmd.Short is empty")
	}

	if rootCmd.Long == "" {
		t.Error("rootCmd.Long is empty")
	}
}

func TestRootCmdVersion(t *testing.T) {
	// Test that version is set (defaults to "dev" if not set at build time)
	if version == "" {
		t.Error("version is empty")
	}

	// Version should be "dev" by default or a semantic version
	if version != "dev" && !isValidVersion(version) {
		t.Errorf("version %q is not a valid version format", version)
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
