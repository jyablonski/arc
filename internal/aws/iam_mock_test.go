package aws

import (
	"fmt"
	"testing"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRunner swaps this package's subprocess seam for the duration of a test and
// restores it afterwards; the swap is scoped to the test via t.Cleanup so
// nothing leaks between tests.
func setRunner(t *testing.T, m boundary.ShellRunner) {
	t.Helper()
	prev := run
	run = m
	t.Cleanup(func() { run = prev })
}

func TestGetCurrentIdentity(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    string
		mockError     error
		expectedError bool
		expectedARN   string
	}{
		{
			name: "successful identity retrieval",
			mockOutput: `{
				"UserId": "AIDAIOSFODNN7EXAMPLE",
				"Account": "123456789012",
				"Arn": "arn:aws:iam::123456789012:user/jacob"
			}`,
			mockError:     nil,
			expectedError: false,
			expectedARN:   "arn:aws:iam::123456789012:user/jacob",
		},
		{
			name:          "aws cli error",
			mockOutput:    "",
			mockError:     fmt.Errorf("aws: command not found"),
			expectedError: true,
		},
		{
			name:          "invalid JSON",
			mockOutput:    "not json",
			mockError:     nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "aws" && len(args) >= 2 && args[0] == "sts" && args[1] == "get-caller-identity" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			setRunner(t, mock)

			identity, err := GetCurrentIdentity()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, identity)

			if tt.expectedARN != "" {
				arn, ok := identity["Arn"].(string)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedARN, arn)
			}
		})
	}
}

func TestListAccessKeys(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		mockOutput    string
		mockError     error
		expectedError bool
		expectedKeys  []string
	}{
		{
			name:     "successful list",
			username: "jacob",
			mockOutput: `{
				"AccessKeyMetadata": [
					{
						"AccessKeyId": "AKIAIOSFODNN7EXAMPLE",
						"Status": "Active",
						"CreateDate": "2023-01-01T00:00:00Z"
					},
					{
						"AccessKeyId": "AKIAEXAMPLE123",
						"Status": "Active",
						"CreateDate": "2023-02-01T00:00:00Z"
					}
				]
			}`,
			mockError:     nil,
			expectedError: false,
			expectedKeys:  []string{"AKIAIOSFODNN7EXAMPLE", "AKIAEXAMPLE123"},
		},
		{
			name:          "empty list",
			username:      "jacob",
			mockOutput:    `{"AccessKeyMetadata": []}`,
			mockError:     nil,
			expectedError: false,
			expectedKeys:  []string{},
		},
		{
			name:          "aws cli error",
			username:      "jacob",
			mockOutput:    "",
			mockError:     fmt.Errorf("aws: command not found"),
			expectedError: true,
		},
		{
			name:          "invalid JSON",
			username:      "jacob",
			mockOutput:    "not json",
			mockError:     nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "aws" && len(args) >= 4 && args[0] == "iam" && args[1] == "list-access-keys" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			setRunner(t, mock)

			keys, err := ListAccessKeys(tt.username)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedKeys, keys)

			// The username must be threaded through to the CLI invocation.
			require.Len(t, mock.RunCalls(), 1)
			assert.Equal(t, []string{"iam", "list-access-keys", "--user-name", tt.username}, mock.RunCalls()[0].Args)
		})
	}
}

func TestCreateAccessKey(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		mockOutput    string
		mockError     error
		expectedError bool
		expectedKeyID string
	}{
		{
			name:     "successful creation",
			username: "jacob",
			mockOutput: `{
				"AccessKey": {
					"AccessKeyId": "AKIAIOSFODNN7EXAMPLE",
					"SecretAccessKey": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
					"Status": "Active",
					"CreateDate": "2023-01-01T00:00:00Z"
				}
			}`,
			mockError:     nil,
			expectedError: false,
			expectedKeyID: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:          "aws cli error",
			username:      "jacob",
			mockOutput:    "",
			mockError:     fmt.Errorf("aws: command not found"),
			expectedError: true,
		},
		{
			name:          "invalid JSON",
			username:      "jacob",
			mockOutput:    "not json",
			mockError:     nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "aws" && len(args) >= 4 && args[0] == "iam" && args[1] == "create-access-key" {
						return tt.mockOutput, tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			setRunner(t, mock)

			keyID, secretKey, err := CreateAccessKey(tt.username)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedKeyID, keyID)
			assert.NotEmpty(t, secretKey)
		})
	}
}

func TestDeleteAccessKey(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		keyID         string
		mockError     error
		expectedError bool
	}{
		{
			name:          "successful deletion",
			username:      "jacob",
			keyID:         "AKIAIOSFODNN7EXAMPLE",
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "aws cli error",
			username:      "jacob",
			keyID:         "AKIAIOSFODNN7EXAMPLE",
			mockError:     fmt.Errorf("aws: command not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &boundary.ShellRunnerMock{
				RunFunc: func(name string, args ...string) (string, error) {
					if name == "aws" && len(args) >= 6 && args[0] == "iam" && args[1] == "delete-access-key" {
						return "", tt.mockError
					}
					return "", fmt.Errorf("unexpected command: %s %v", name, args)
				},
			}
			setRunner(t, mock)

			err := DeleteAccessKey(tt.username, tt.keyID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
