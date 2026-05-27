package setupdeps

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/shell"
)

var ErrUnsupportedPlatform = errors.New("setup is not supported on this platform")

type Installer interface {
	Install() error
}

func New(os platform.OS) Installer {
	switch os {
	case platform.Linux:
		return linuxInstaller{}
	case platform.Darwin:
		return darwinInstaller{}
	default:
		return unsupportedInstaller{}
	}
}

type linuxInstaller struct{}

func (linuxInstaller) Install() error {
	if !shell.CommandExists("pacman") {
		return shell.NewErrToolNotAvailable("pacman")
	}

	packagesToInstall := []struct {
		name        string
		description string
		installCmd  []string
		checkCmd    string
	}{
		{name: "github-cli", description: "GitHub CLI (gh)", installCmd: []string{"sudo", "pacman", "-S", "--noconfirm", "github-cli"}, checkCmd: "gh"},
		{name: "dmidecode", description: "DMI decode utility (for hardware info)", installCmd: []string{"sudo", "pacman", "-S", "--noconfirm", "dmidecode"}, checkCmd: "dmidecode"},
		{name: "lshw", description: "Hardware lister (for RAM info)", installCmd: []string{"sudo", "pacman", "-S", "--noconfirm", "lshw"}, checkCmd: "lshw"},
		{name: "git", description: "Git version control", installCmd: []string{"sudo", "pacman", "-S", "--noconfirm", "git"}, checkCmd: "git"},
		{name: "fastfetch", description: "System info tool (for arc info)", installCmd: []string{"sudo", "pacman", "-S", "--noconfirm", "fastfetch"}, checkCmd: "fastfetch"},
	}

	for _, pkg := range packagesToInstall {
		if shell.CommandExists(pkg.checkCmd) {
			output.Info(fmt.Sprintf("%s (%s) is already installed", pkg.name, pkg.description))
			continue
		}
		output.Info(fmt.Sprintf("Installing %s (%s)...", pkg.name, pkg.description))
		if err := shell.RunInteractive(pkg.installCmd[0], pkg.installCmd[1:]...); err != nil {
			output.Warning(fmt.Sprintf("Failed to install %s: %v", pkg.name, err))
		} else {
			output.Success(fmt.Sprintf("Installed %s", pkg.name))
		}
	}

	if !shell.CommandExists("uv") {
		output.Info("Installing uv (Python package manager)...")
		curlCmd := exec.Command("sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
		curlCmd.Stdin = os.Stdin
		curlCmd.Stdout = os.Stdout
		curlCmd.Stderr = os.Stderr
		if err := curlCmd.Run(); err != nil {
			output.Warning(fmt.Sprintf("Failed to install uv: %v", err))
			output.Info("You may need to add ~/.cargo/bin to your PATH")
		} else {
			output.Success("Installed uv")
			if shell.CommandExists("uv") {
				output.Success("uv is available in PATH")
			} else {
				output.Warning("uv was installed but is not in PATH")
				output.Info("Add this to your ~/.zshrc or ~/.bashrc:")
				output.Info("  export PATH=\"$HOME/.cargo/bin:$PATH\"")
				output.Info("Then restart your shell or run: source ~/.zshrc")
			}
		}
	} else {
		output.Info("uv is already installed")
	}

	return nil
}

type darwinInstaller struct{}

func (darwinInstaller) Install() error {
	if !shell.CommandExists("brew") {
		output.Error("Homebrew is required for macOS setup but is not installed")
		output.Info("Install Homebrew manually, then rerun 'arc setup':")
		output.Info(`  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
		return shell.NewErrToolNotAvailable("brew")
	}

	packagesToInstall := []struct {
		name        string
		description string
		checkCmd    string
	}{
		{name: "gh", description: "GitHub CLI", checkCmd: "gh"},
		{name: "git", description: "Git version control", checkCmd: "git"},
		{name: "fastfetch", description: "System info tool (for arc info)", checkCmd: "fastfetch"},
		{name: "uv", description: "Python package manager", checkCmd: "uv"},
	}

	for _, pkg := range packagesToInstall {
		if shell.CommandExists(pkg.checkCmd) {
			output.Info(fmt.Sprintf("%s (%s) is already installed", pkg.name, pkg.description))
			continue
		}
		output.Info(fmt.Sprintf("Installing %s (%s)...", pkg.name, pkg.description))
		if err := shell.RunInteractive("brew", "install", pkg.name); err != nil {
			output.Warning(fmt.Sprintf("Failed to install %s: %v", pkg.name, err))
		} else {
			output.Success(fmt.Sprintf("Installed %s", pkg.name))
		}
	}

	return nil
}

type unsupportedInstaller struct{}

func (unsupportedInstaller) Install() error {
	return ErrUnsupportedPlatform
}
