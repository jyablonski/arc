package aws

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUsername(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
		wantErr  bool
	}{
		{
			name:     "standard IAM user ARN",
			arn:      "arn:aws:iam::123456789012:user/jacob",
			expected: "jacob",
			wantErr:  false,
		},
		{
			name:     "IAM user with path",
			arn:      "arn:aws:iam::123456789012:user/path/to/user",
			expected: "user",
			wantErr:  false,
		},
		{
			name:     "root account",
			arn:      "arn:aws:iam::123456789012:root",
			expected: "arn:aws:iam::123456789012:root", // No slash, returns whole string
			wantErr:  false,
		},
		{
			name:     "role ARN",
			arn:      "arn:aws:iam::123456789012:role/MyRole",
			expected: "MyRole",
			wantErr:  false,
		},
		{
			name:     "assumed role",
			arn:      "arn:aws:sts::123456789012:assumed-role/MyRole/session",
			expected: "session",
			wantErr:  false,
		},
		{
			name:     "invalid ARN - empty",
			arn:      "",
			expected: "",
			wantErr:  false, // Current implementation doesn't error on empty
		},
		{
			name:     "invalid ARN - no slash",
			arn:      "arn:aws:iam::123456789012:user",
			expected: "arn:aws:iam::123456789012:user", // No slash, returns whole string
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetUsername(tt.arn)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateCredentialsWith_success(t *testing.T) {
	out, err := validateCredentialsWith("AKIA1234567890", "secret", 1, func(env []string) (string, string, error) {
		var hasKey, hasSecret bool
		for _, e := range env {
			if e == "AWS_ACCESS_KEY_ID=AKIA1234567890" {
				hasKey = true
			}
			if strings.TrimPrefix(e, "AWS_SECRET_ACCESS_KEY=") == "secret" {
				hasSecret = true
			}
		}
		require.True(t, hasKey)
		require.True(t, hasSecret)
		return `{"Account":"111"}`, "", nil
	})
	require.NoError(t, err)
	require.Contains(t, out, "111")
}

func TestValidateCredentialsWith_failureUsesStderr(t *testing.T) {
	_, err := validateCredentialsWith("x", "y", 1, func([]string) (string, string, error) {
		return "", "InvalidClientTokenId", errors.New("exit status 255")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "InvalidClientTokenId")
}

func TestValidateCredentialsWith_failureFallsBackToErrError(t *testing.T) {
	_, err := validateCredentialsWith("x", "y", 1, func([]string) (string, string, error) {
		return "", "", fmt.Errorf("boom")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestValidateCredentials_publicUsesStsCaller(t *testing.T) {
	prev := stsCaller
	t.Cleanup(func() { stsCaller = prev })
	stsCaller = func(env []string) (string, string, error) {
		var saw bool
		for _, e := range env {
			if strings.Contains(e, "AWS_ACCESS_KEY_ID=AKIA") {
				saw = true
				break
			}
		}
		require.True(t, saw)
		return `{"Account":"999"}`, "", nil
	}
	out, err := ValidateCredentials("AKIA", "secretkey", 1)
	require.NoError(t, err)
	require.Contains(t, out, "999")
}
