// Package statepath resolves Arc's persistent state directory.
package statepath

import (
	"os"
	"path/filepath"
	"runtime"
)

// ArcDir returns the platform-appropriate Arc state directory, honoring
// XDG_STATE_HOME when it is set.
func ArcDir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "arc"), nil
	}
	if runtime.GOOS == "darwin" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "arc"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "arc"), nil
}
