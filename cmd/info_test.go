package cmd

import (
	"testing"

	"github.com/jyablonski/arc/internal/shell"
	"github.com/stretchr/testify/assert"
)

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "standard arch os-release",
			content: `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch
BUILD_ID=rolling
ANSI_COLOR="38;2;23;147;209"`,
			expected: "Arch Linux",
		},
		{
			name: "ubuntu os-release with version",
			content: `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
PRETTY_NAME="Ubuntu 22.04.3 LTS"
ID=ubuntu`,
			expected: "Ubuntu 22.04.3 LTS",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name: "no PRETTY_NAME line",
			content: `NAME="Arch Linux"
ID=arch`,
			expected: "",
		},
		{
			name:     "PRETTY_NAME with no value",
			content:  "PRETTY_NAME=",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOSRelease(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDisplayServer(t *testing.T) {
	tests := []struct {
		name        string
		sessionType string
		expected    string
	}{
		{
			name:        "wayland session",
			sessionType: "wayland",
			expected:    "Wayland (active)",
		},
		{
			name:        "x11 session",
			sessionType: "x11",
			expected:    "X11 (active)",
		},
		{
			name:        "empty session type",
			sessionType: "",
			expected:    "Unknown: ",
		},
		{
			name:        "unexpected session type",
			sessionType: "mir",
			expected:    "Unknown: mir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDisplayServer(tt.sessionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGatherSystemInfo(t *testing.T) {
	tests := []struct {
		name     string
		mockRun  func(name string, args ...string) (string, error)
		expected SystemInfo
	}{
		{
			name: "all commands succeed",
			mockRun: func(name string, args ...string) (string, error) {
				switch name {
				case "cat":
					return `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch`, nil
				case "uname":
					return "6.7.0-arch1-1", nil
				case "gnome-shell":
					return "GNOME Shell 45.3", nil
				case "nvidia-smi":
					return "545.29.06", nil
				default:
					return "", nil
				}
			},
			expected: SystemInfo{
				OS:           "Arch Linux",
				Kernel:       "6.7.0-arch1-1",
				GNOMEVersion: "GNOME Shell 45.3",
				NVIDIADriver: "545.29.06",
			},
		},
		{
			name: "nvidia not installed",
			mockRun: func(name string, args ...string) (string, error) {
				switch name {
				case "cat":
					return `PRETTY_NAME="Arch Linux"`, nil
				case "uname":
					return "6.7.0-arch1-1", nil
				case "gnome-shell":
					return "GNOME Shell 45.3", nil
				case "nvidia-smi":
					return "", assert.AnError
				default:
					return "", nil
				}
			},
			expected: SystemInfo{
				OS:           "Arch Linux",
				Kernel:       "6.7.0-arch1-1",
				GNOMEVersion: "GNOME Shell 45.3",
				NVIDIADriver: "Not installed or not detected",
			},
		},
		{
			name: "all commands fail",
			mockRun: func(name string, args ...string) (string, error) {
				return "", assert.AnError
			},
			expected: SystemInfo{
				NVIDIADriver: "Not installed or not detected",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &shell.MockRunner{
				RunFunc: tt.mockRun,
			}
			shell.SetMockRunner(mock)
			defer shell.ClearMockRunner()

			// DisplayServer reads from os.Getenv which we don't mock here,
			// so just check the other fields
			info := gatherSystemInfo()
			assert.Equal(t, tt.expected.OS, info.OS)
			assert.Equal(t, tt.expected.Kernel, info.Kernel)
			assert.Equal(t, tt.expected.GNOMEVersion, info.GNOMEVersion)
			assert.Equal(t, tt.expected.NVIDIADriver, info.NVIDIADriver)
		})
	}
}
