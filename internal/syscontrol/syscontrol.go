package syscontrol

import (
	"errors"
	"fmt"

	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
)

var ErrUnsupportedPlatform = errors.New("system control is not supported on this platform")

type Controller interface {
	Sleep() error
}

func New(os platform.OS) Controller {
	switch os {
	case platform.Linux:
		return linuxController{}
	case platform.Darwin:
		return darwinController{}
	default:
		return unsupportedController{}
	}
}

type linuxController struct{}

func (linuxController) Sleep() error {
	if _, err := shell.RunSudo("systemctl", "suspend"); err != nil {
		return fmt.Errorf("failed to suspend: %w", err)
	}
	return nil
}

type darwinController struct{}

func (darwinController) Sleep() error {
	if _, err := shell.Run("pmset", "sleepnow"); err != nil {
		return fmt.Errorf("failed to suspend: %w", err)
	}
	return nil
}

type unsupportedController struct{}

func (unsupportedController) Sleep() error {
	return ErrUnsupportedPlatform
}
