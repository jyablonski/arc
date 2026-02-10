package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
