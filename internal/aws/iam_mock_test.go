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
