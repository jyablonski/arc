package sysupdate

import (
	"fmt"
	"time"

	"github.com/jyablonski/arc/internal/output"
)

// RunWithDeps runs the pacman (and optional yay / cache) update flow.
func RunWithDeps(deps Deps, opts Options) error {
	d := mergeDeps(deps)

	now := time.Now()
	output.Header(fmt.Sprintf("Starting system update - %s", now.Format("2006-01-02 15:04:05")))

	if err := d.CheckPacman(); err != nil {
		return err
	}

	output.Info("Updating archlinux-keyring...")
	if err := d.RunInteractive("sudo", "pacman", "-Sy", "--needed", "--noconfirm", "archlinux-keyring"); err != nil {
		return fmt.Errorf("keyring update failed: %w", err)
	}

	kernelPackagesBefore, err := d.KernelVersions()
	if err != nil {
		output.Warning(fmt.Sprintf("Failed to get kernel packages before update: %v", err))
		kernelPackagesBefore = make(map[string]string)
	}

	output.Info("Running pacman -Syu...")
	if err := d.RunInteractive("sudo", "pacman", "-Syu", "--noconfirm"); err != nil {
		return fmt.Errorf("pacman update failed: %w", err)
	}

	kernelPackagesAfter, err := d.KernelVersions()
	if err != nil {
		output.Warning(fmt.Sprintf("Failed to get kernel packages after update: %v", err))
		kernelPackagesAfter = make(map[string]string)
	}

	if updated, msg := kernelChangeMessage(kernelPackagesBefore, kernelPackagesAfter); updated {
		output.Warning(msg)
		if err := promptReboot(d.Stdin, d.RunInteractive); err != nil {
			return err
		}
	}

	if !opts.SkipAUR {
		if d.CheckYayAvailable() {
			output.Info("Running yay -Syu --aur...")
			if err := d.RunInteractive("yay", "-Syu", "--aur", "--noconfirm", "--answerclean=None", "--answerdiff=None", "--answeredit=None"); err != nil {
				output.Warning(fmt.Sprintf("yay update failed: %v", err))
			}
		} else {
			output.Warning("yay is not available, skipping AUR updates")
		}
	}

	if !opts.SkipCache {
		output.Info("Cleaning package cache...")
		if _, err := d.RunSudo("paccache", "-rv"); err != nil {
			output.Warning(fmt.Sprintf("paccache failed: %v", err))
		}
	}

	output.Header(fmt.Sprintf("System update finished - %s", time.Now().Format("2006-01-02 15:04:05")))
	return nil
}
