package pacman

import (
	"errors"
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestCheckAvailable(t *testing.T) {
	t.Run("When checking pacman, it returns without panicking", func(t *testing.T) {
		err := CheckPacmanAvailable()
		// This test depends on the system - pacman might or might not be available
		if err != nil {
			var toolErr *shell.ErrToolNotAvailable
			assert.True(t, errors.As(err, &toolErr))
			assert.Equal(t, "pacman", toolErr.Tool)
		}
	})

	t.Run("When checking yay, it returns without panicking", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = CheckYayAvailable()
		})
	})
}
