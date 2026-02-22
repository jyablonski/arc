package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install required packages and tools",
	Long: `Install required packages and tools needed for arc to function properly.
This includes uv, gh (GitHub CLI), and other system utilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Header("Setting up arc dependencies")

		// Check if we're on Arch Linux
		if !shell.CommandExists("pacman") {
			return shell.NewErrToolNotAvailable("pacman")
		}

		packagesToInstall := []struct {
			name        string
			description string
			installCmd  []string
			checkCmd    string
		}{
			{
				name:        "github-cli",
				description: "GitHub CLI (gh)",
				installCmd:  []string{"sudo", "pacman", "-S", "--noconfirm", "github-cli"},
				checkCmd:    "gh",
			},
			{
				name:        "dmidecode",
				description: "DMI decode utility (for hardware info)",
				installCmd:  []string{"sudo", "pacman", "-S", "--noconfirm", "dmidecode"},
				checkCmd:    "dmidecode",
			},
			{
				name:        "lshw",
				description: "Hardware lister (for RAM info)",
				installCmd:  []string{"sudo", "pacman", "-S", "--noconfirm", "lshw"},
				checkCmd:    "lshw",
			},
			{
				name:        "git",
				description: "Git version control",
				installCmd:  []string{"sudo", "pacman", "-S", "--noconfirm", "git"},
				checkCmd:    "git",
			},
		}

		// Install packages via pacman
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

		// Install uv via curl script
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
				// Check if uv is now available (might be in ~/.cargo/bin)
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

		output.Header("Setup complete")
		output.Info("Run 'arc validate' to check if all dependencies are available")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
