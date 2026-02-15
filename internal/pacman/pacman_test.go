package pacman

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPacmanAvailable(t *testing.T) {
	err := CheckPacmanAvailable()
	// This test depends on the system - pacman might or might not be available
	// We just check that it doesn't panic
	if err != nil {
		assert.Equal(t, "pacman is not available in PATH", err.Error())
	}
}

func TestCheckYayAvailable(t *testing.T) {
	// This test depends on the system - yay might or might not be available
	// We just check that it returns a boolean and doesn't panic
	assert.NotPanics(t, func() {
		_ = CheckYayAvailable()
	})
}
