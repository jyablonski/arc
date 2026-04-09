package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtraCommandsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		value    *string
		expected bool
	}{
		{
			name:     "unset defaults to disabled",
			value:    nil,
			expected: false,
		},
		{
			name:     "one enables extra commands",
			value:    ptr("1"),
			expected: true,
		},
		{
			name:     "true enables extra commands",
			value:    ptr("true"),
			expected: true,
		},
		{
			name:     "false disables extra commands",
			value:    ptr("false"),
			expected: false,
		},
		{
			name:     "invalid value disables extra commands",
			value:    ptr("gh restart-dashboard"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue, hadValue := os.LookupEnv(extraCommandsEnvVar)
			defer func() {
				if hadValue {
					_ = os.Setenv(extraCommandsEnvVar, oldValue)
				} else {
					_ = os.Unsetenv(extraCommandsEnvVar)
				}
			}()

			if tt.value == nil {
				_ = os.Unsetenv(extraCommandsEnvVar)
			} else {
				_ = os.Setenv(extraCommandsEnvVar, *tt.value)
			}

			assert.Equal(t, tt.expected, extraCommandsEnabled())
		})
	}
}

func ptr(s string) *string {
	return &s
}
