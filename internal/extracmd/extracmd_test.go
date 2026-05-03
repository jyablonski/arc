package extracmd_test

import (
	"os"
	"testing"

	"github.com/jyablonski/arc/internal/extracmd"
	"github.com/stretchr/testify/assert"
)

func TestEnabled(t *testing.T) {
	tests := []struct {
		name     string
		value    *string
		expected bool
	}{
		{name: "unset defaults to disabled", value: nil, expected: false},
		{name: "one enables extra commands", value: ptr("1"), expected: true},
		{name: "true enables extra commands", value: ptr("true"), expected: true},
		{name: "false disables extra commands", value: ptr("false"), expected: false},
		{name: "invalid value disables extra commands", value: ptr("gh restart-dashboard"), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue, hadValue := os.LookupEnv(extracmd.EnvVar)
			defer func() {
				if hadValue {
					_ = os.Setenv(extracmd.EnvVar, oldValue)
				} else {
					_ = os.Unsetenv(extracmd.EnvVar)
				}
			}()

			if tt.value == nil {
				_ = os.Unsetenv(extracmd.EnvVar)
			} else {
				_ = os.Setenv(extracmd.EnvVar, *tt.value)
			}

			assert.Equal(t, tt.expected, extracmd.Enabled())
		})
	}
}

func ptr(s string) *string {
	return &s
}
