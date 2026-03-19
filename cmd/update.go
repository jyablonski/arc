package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var (
	updateNoAUR   bool
	updateNoCache bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Run system updates (pacman, yay, cache cleanup)",
	Long: `Update the system by running pacman -Syu, optionally yay -Syu --aur,
and cleaning the package cache with paccache -rv.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now()
		output.Header(fmt.Sprintf("hello jacob, running updates for %s", now.Format("2006-01-02 15:04:05")))

		// Check if pacman is available
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		// Update keyring first to avoid signature issues
		output.Info("Updating archlinux-keyring...")
		if err := shell.RunInteractive("sudo", "pacman", "-Sy", "--needed", "--noconfirm", "archlinux-keyring"); err != nil {
			return fmt.Errorf("keyring update failed: %w", err)
		}

		// Get kernel packages before update
		kernelPackagesBefore, err := getKernelPackages()
		if err != nil {
			output.Warning(fmt.Sprintf("Failed to get kernel packages before update: %v", err))
			kernelPackagesBefore = make(map[string]string)
		}

		// Run pacman update
		output.Info("Running pacman -Syu...")
		if err := shell.RunInteractive("sudo", "pacman", "-Syu", "--noconfirm"); err != nil {
			return fmt.Errorf("pacman update failed: %w", err)
		}

		// Check if kernel was updated
		kernelPackagesAfter, err := getKernelPackages()
		if err != nil {
			output.Warning(fmt.Sprintf("Failed to get kernel packages after update: %v", err))
			kernelPackagesAfter = make(map[string]string)
		}

		// Check if any kernel package was updated
		kernelUpdated := false
		for pkg, versionAfter := range kernelPackagesAfter {
			if versionBefore, exists := kernelPackagesBefore[pkg]; exists {
				if versionBefore != versionAfter {
					kernelUpdated = true
					output.Warning(fmt.Sprintf("Kernel package %s was updated from %s to %s", pkg, versionBefore, versionAfter))
					break
				}
			} else {
				// New kernel package installed
				kernelUpdated = true
				output.Warning(fmt.Sprintf("New kernel package %s was installed (version %s)", pkg, versionAfter))
				break
			}
		}

		// Prompt for reboot if kernel was updated
		if kernelUpdated {
			if err := promptReboot(); err != nil {
				return err
			}
		}

		// Run yay update if not disabled
		if !updateNoAUR {
			if pacman.CheckYayAvailable() {
				output.Info("Running yay -Syu --aur...")
				if err := shell.RunInteractive("yay", "-Syu", "--aur", "--noconfirm", "--nocleanmenu", "--nodiffmenu", "--noeditmenu"); err != nil {
					output.Warning(fmt.Sprintf("yay update failed: %v", err))
				}
			} else {
				output.Warning("yay is not available, skipping AUR updates")
			}
		}

		// Clean package cache if not disabled
		if !updateNoCache {
			output.Info("Cleaning package cache...")
			if _, err := shell.RunSudo("paccache", "-rv"); err != nil {
				output.Warning(fmt.Sprintf("paccache failed: %v", err))
			}
		}

		output.Header(fmt.Sprintf("updates complete at %s, hasta la vista", time.Now().Format("2006-01-02 15:04:05")))
		return nil
	},
}

var updateUvCmd = &cobra.Command{
	Use:   "uv",
	Short: "Update uv package manager",
	Long:  `Update uv to the latest version using uv self update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("uv") {
			return shell.NewErrToolNotAvailable("uv")
		}

		output.Info("Updating uv...")
		if err := shell.RunInteractive("uv", "self", "update"); err != nil {
			return fmt.Errorf("failed to update uv: %w", err)
		}

		output.Success("uv updated successfully")
		return nil
	},
}

// getKernelPackages returns a map of kernel package names to their versions
func getKernelPackages() (map[string]string, error) {
	output, err := shell.Run("pacman", "-Q")
	if err != nil {
		return nil, err
	}

	kernelPackages := make(map[string]string)
	kernelPatterns := []string{"linux ", "linux-lts ", "linux-zen ", "linux-hardened ", "linux-headers ", "linux-lts-headers ", "linux-zen-headers ", "linux-hardened-headers "}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		for _, pattern := range kernelPatterns {
			if strings.HasPrefix(line, pattern) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					pkgName := parts[0]
					version := parts[1]
					// Only track main kernel packages, not headers
					if !strings.Contains(pkgName, "-headers") {
						kernelPackages[pkgName] = version
					}
				}
				break
			}
		}
	}

	return kernelPackages, nil
}

// promptReboot prompts the user to reboot and executes reboot if confirmed
func promptReboot() error {
	output.Warning("A kernel update was successfully installed. A reboot is required for the changes to take effect.")
	fmt.Print("Reboot now? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" || response == "y" || response == "yes" {
		output.Info("Rebooting now...")
		if err := shell.RunInteractive("sudo", "reboot"); err != nil {
			return fmt.Errorf("failed to reboot: %w", err)
		}
	} else {
		output.Info("Reboot skipped. Please reboot manually when convenient.")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateNoAUR, "no-aur", false, "Skip AUR updates")
	updateCmd.Flags().BoolVar(&updateNoCache, "no-cache", false, "Skip cache cleanup")
	updateCmd.AddCommand(updateUvCmd)
}
