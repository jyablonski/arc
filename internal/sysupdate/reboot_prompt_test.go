package sysupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRebootConfirmation(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldReboot bool
	}{
		{name: "yes response", input: "y\n", shouldReboot: true},
		{name: "yes uppercase", input: "Y\n", shouldReboot: true},
		{name: "yes full word", input: "yes\n", shouldReboot: true},
		{name: "empty response (default yes)", input: "\n", shouldReboot: true},
		{name: "no response", input: "n\n", shouldReboot: false},
		{name: "no uppercase", input: "N\n", shouldReboot: false},
		{name: "other response", input: "maybe\n", shouldReboot: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.shouldReboot, parseRebootConfirmation(tt.input))
		})
	}
}
