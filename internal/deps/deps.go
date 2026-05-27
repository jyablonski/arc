package deps

import "github.com/jyablonski/arc/internal/platform"

type ToolStatus struct {
	Name        string
	Required    bool
	Available   bool
	Description string
}

func Tools(os platform.OS) []ToolStatus {
	switch os {
	case platform.Darwin:
		return []ToolStatus{
			{Name: "brew", Required: true, Description: "Homebrew package manager"},
			{Name: "git", Required: true, Description: "Git version control"},
			{Name: "gh", Required: true, Description: "GitHub CLI"},
			{Name: "fastfetch", Required: true, Description: "System info tool (for arc info)"},
			{Name: "uv", Required: true, Description: "Python package manager"},
			{Name: "system_profiler", Required: true, Description: "macOS system profiler (for hardware info)"},
			{Name: "pmset", Required: true, Description: "macOS power management (for arc sleep)"},
			{Name: "sysctl", Required: true, Description: "macOS system control info"},
			{Name: "docker", Required: false, Description: "Docker (for docker clean command)"},
			{Name: "aws", Required: false, Description: "AWS CLI (for AWS commands)"},
		}
	case platform.Linux:
		return []ToolStatus{
			{Name: "pacman", Required: true, Description: "Package manager (base system)"},
			{Name: "systemctl", Required: true, Description: "Systemd control (base system)"},
			{Name: "lspci", Required: true, Description: "PCI device lister (pciutils, base system)"},
			{Name: "dmidecode", Required: true, Description: "DMI decode utility (for hardware info)"},
			{Name: "lshw", Required: true, Description: "Hardware lister (for RAM info)"},
			{Name: "git", Required: true, Description: "Git version control"},
			{Name: "gh", Required: true, Description: "GitHub CLI"},
			{Name: "fastfetch", Required: true, Description: "System info tool (for arc info)"},
			{Name: "uv", Required: true, Description: "Python package manager"},
			{Name: "yay", Required: false, Description: "AUR helper (for arc update system)"},
			{Name: "docker", Required: false, Description: "Docker (for docker clean command)"},
			{Name: "aws", Required: false, Description: "AWS CLI (for AWS commands)"},
			{Name: "nvidia-smi", Required: false, Description: "NVIDIA driver (for GPU info)"},
			{Name: "paccache", Required: false, Description: "Package cache cleaner (arc update system)"},
		}
	default:
		return []ToolStatus{
			{Name: "git", Required: true, Description: "Git version control"},
			{Name: "gh", Required: false, Description: "GitHub CLI"},
			{Name: "uv", Required: false, Description: "Python package manager"},
			{Name: "docker", Required: false, Description: "Docker (for docker clean command)"},
			{Name: "aws", Required: false, Description: "AWS CLI (for AWS commands)"},
		}
	}
}
