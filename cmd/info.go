package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

type SystemInfo struct {
	OS            string `json:"os"`
	Kernel        string `json:"kernel"`
	GNOMEVersion  string `json:"gnome_version"`
	NVIDIADriver  string `json:"nvidia_driver"`
	DisplayServer string `json:"display_server"`
}

var infoJSON bool

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show system information",
	Long: `Display system information including OS version, kernel version,
GNOME version, NVIDIA driver version, and display server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := gatherSystemInfo()

		if infoJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(info)
		}

		output.Header("=== System Information ===")
		fmt.Println()
		fmt.Println(info.OS)
		fmt.Println()
		fmt.Println("Kernel:")
		fmt.Println(info.Kernel)
		fmt.Println()
		fmt.Println("GNOME Version:")
		fmt.Println(info.GNOMEVersion)
		fmt.Println()
		fmt.Println("NVIDIA Driver:")
		fmt.Println(info.NVIDIADriver)
		fmt.Println()
		fmt.Println("Display Server:")
		fmt.Println(info.DisplayServer)
		fmt.Println()

		return nil
	},
}

// gatherSystemInfo collects system information from shell commands and environment
func gatherSystemInfo() SystemInfo {
	info := SystemInfo{}

	// Get OS info
	osRelease, err := shell.Run("cat", "/etc/os-release")
	if err == nil {
		info.OS = parseOSRelease(osRelease)
	}

	// Get kernel version
	kernel, err := shell.Run("uname", "-r")
	if err == nil {
		info.Kernel = kernel
	}

	// Get GNOME version
	gnome, err := shell.Run("gnome-shell", "--version")
	if err == nil {
		info.GNOMEVersion = strings.TrimSpace(gnome)
	}

	// Get NVIDIA driver version
	nvidia, err := shell.Run("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")
	if err == nil {
		info.NVIDIADriver = strings.TrimSpace(nvidia)
	} else {
		info.NVIDIADriver = "Not installed or not detected"
	}

	// Get display server
	info.DisplayServer = parseDisplayServer(os.Getenv("XDG_SESSION_TYPE"))

	return info
}

// parseOSRelease extracts the PRETTY_NAME from /etc/os-release content
func parseOSRelease(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				return strings.Trim(parts[1], "\"")
			}
		}
	}
	return ""
}

// parseDisplayServer returns a display server description from XDG_SESSION_TYPE
func parseDisplayServer(sessionType string) string {
	switch sessionType {
	case "wayland":
		return "Wayland (active)"
	case "x11":
		return "X11 (active)"
	default:
		return fmt.Sprintf("Unknown: %s", sessionType)
	}
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output in JSON format")
}
