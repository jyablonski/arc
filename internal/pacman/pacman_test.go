package pacman

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestCheckPacmanAvailable(t *testing.T) {
	err := CheckPacmanAvailable()
	// This test depends on the system - pacman might or might not be available
	// We just check that it doesn't panic
	if err != nil {
		var toolErr *shell.ErrToolNotAvailable
		assert.True(t, errors.As(err, &toolErr))
		assert.Equal(t, "pacman", toolErr.Tool)
	}
}

func TestCheckYayAvailable(t *testing.T) {
	// This test depends on the system - yay might or might not be available
	// We just check that it returns a boolean and doesn't panic
	assert.NotPanics(t, func() {
		_ = CheckYayAvailable()
	})
}
