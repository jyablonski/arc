package sysupdate

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
)

type Options struct {
	SkipAUR   bool
	SkipCache bool
}

func Run(opts Options) error {
	now := time.Now()
	output.Header(fmt.Sprintf("hello jacob, running updates for %s", now.Format("2006-01-02 15:04:05")))

	if err := pacman.CheckPacmanAvailable(); err != nil {
		return err
	}

	output.Info("Updating archlinux-keyring...")
	if err := shell.RunInteractive("sudo", "pacman", "-Sy", "--needed", "--noconfirm", "archlinux-keyring"); err != nil {
		return fmt.Errorf("keyring update failed: %w", err)
	}

	kernelPackagesBefore, err := pacman.InstalledKernelVersions()
	if err != nil {
		output.Warning(fmt.Sprintf("Failed to get kernel packages before update: %v", err))
		kernelPackagesBefore = make(map[string]string)
	}

	output.Info("Running pacman -Syu...")
	if err := shell.RunInteractive("sudo", "pacman", "-Syu", "--noconfirm"); err != nil {
		return fmt.Errorf("pacman update failed: %w", err)
	}

	kernelPackagesAfter, err := pacman.InstalledKernelVersions()
	if err != nil {
		output.Warning(fmt.Sprintf("Failed to get kernel packages after update: %v", err))
		kernelPackagesAfter = make(map[string]string)
	}

	kernelUpdated := false
	for pkg, versionAfter := range kernelPackagesAfter {
		if versionBefore, exists := kernelPackagesBefore[pkg]; exists {
			if versionBefore != versionAfter {
				kernelUpdated = true
				output.Warning(fmt.Sprintf("Kernel package %s was updated from %s to %s", pkg, versionBefore, versionAfter))
				break
			}
		} else {
			kernelUpdated = true
			output.Warning(fmt.Sprintf("New kernel package %s was installed (version %s)", pkg, versionAfter))
			break
		}
	}

	if kernelUpdated {
		if err := PromptReboot(os.Stdin); err != nil {
			return err
		}
	}

	if !opts.SkipAUR {
		if pacman.CheckYayAvailable() {
			output.Info("Running yay -Syu --aur...")
			if err := shell.RunInteractive("yay", "-Syu", "--aur", "--noconfirm", "--answerclean=None", "--answerdiff=None", "--answeredit=None"); err != nil {
				output.Warning(fmt.Sprintf("yay update failed: %v", err))
			}
		} else {
			output.Warning("yay is not available, skipping AUR updates")
		}
	}

	if !opts.SkipCache {
		output.Info("Cleaning package cache...")
		if _, err := shell.RunSudo("paccache", "-rv"); err != nil {
			output.Warning(fmt.Sprintf("paccache failed: %v", err))
		}
	}

	output.Header(fmt.Sprintf("updates complete at %s, hasta la vista", time.Now().Format("2006-01-02 15:04:05")))
	return nil
}

func PromptReboot(r *os.File) error {
	output.Warning("A kernel update was successfully installed. A reboot is required for the changes to take effect.")
	fmt.Print("Reboot now? [Y/n]: ")

	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if parseRebootConfirmation(response) {
		output.Info("Rebooting now...")
		if err := shell.RunInteractive("sudo", "reboot"); err != nil {
			return fmt.Errorf("failed to reboot: %w", err)
		}
	} else {
		output.Info("Reboot skipped. Please reboot manually when convenient.")
	}

	return nil
}
