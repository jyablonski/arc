package sysupdate

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
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

	// Note a kernel change now (so the warning is timely) but defer the reboot
	// prompt to the very end: rebooting here would skip AUR updates and cache
	// cleanup. Reboot last, after all update work is done.
	rebootNeeded, rebootMsg := kernelChangeMessage(kernelPackagesBefore, kernelPackagesAfter)
	if rebootNeeded {
		output.Warning(rebootMsg)
	}

	if !opts.SkipAUR {
		if d.CheckYayAvailable() {
			// Triage before yay builds anything; the baseline is committed only
			// if yay succeeds, so a takeover you reject at the diffmenu stays
			// flagged on the next run.
			result := runAURReview(d)
			output.Info("Running yay -Syu --aur with diff and PKGBUILD review...")
			if err := d.RunInteractive("yay", "-Syu", "--aur", "--diffmenu", "--editmenu", "--noanswerdiff", "--noansweredit"); err != nil {
				output.Warning(fmt.Sprintf("yay update failed: %v", err))
			} else if result != nil && d.CommitAUR != nil {
				if err := d.CommitAUR(result); err != nil {
					output.Warning(fmt.Sprintf("AUR provenance baseline not saved: %v", err))
				}
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

	if rebootNeeded {
		return promptReboot(d.Stdin, d.RunInteractive)
	}
	return nil
}

// runAURReview fetches AUR metadata and prints triage findings before yay
// runs. It never blocks the update: any failure is reported and skipped. The
// returned result carries the baseline to commit if yay succeeds.
func runAURReview(d Deps) *aurreview.Result {
	if d.ForeignPackages == nil || d.ReviewAUR == nil {
		return nil
	}
	installed, err := d.ForeignPackages()
	if err != nil {
		output.Warning(fmt.Sprintf("AUR review unavailable: %v", err))
		return nil
	}
	installed = excludeIgnored(d, installed)
	if len(installed) == 0 {
		output.Info("No AUR packages to review")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	result, err := d.ReviewAUR(ctx, installed)
	if err != nil {
		output.Warning(fmt.Sprintf("AUR review unavailable: %v", err))
		return nil
	}
	printAURReview(result)
	return result
}

// excludeIgnored drops packages matched by pacman.conf IgnorePkg (exact or
// glob) so arc doesn't triage updates yay won't apply. Read failures are
// reported and treated as "nothing ignored" rather than blocking the review.
func excludeIgnored(d Deps, installed map[string]string) map[string]string {
	if d.IgnoredPackages == nil {
		return installed
	}
	patterns, err := d.IgnoredPackages()
	if err != nil {
		output.Warning(fmt.Sprintf("could not read ignored packages: %v", err))
		return installed
	}
	if len(patterns) == 0 {
		return installed
	}
	out := make(map[string]string, len(installed))
	for name, ver := range installed {
		if !matchesAnyPattern(name, patterns) {
			out[name] = ver
		}
	}
	return out
}

func matchesAnyPattern(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == name {
			return true
		}
		if ok, err := filepath.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}

func printAURReview(result *aurreview.Result) {
	if len(result.Findings) == 0 {
		if result.Pending == 0 {
			output.Info("No AUR updates pending")
		} else {
			output.Success(fmt.Sprintf("AUR review: %d update(s) pending, no notable findings", result.Pending))
		}
		return
	}
	output.Info("AUR review findings (most severe first):")
	for _, f := range result.Findings {
		line := fmt.Sprintf("[%s] %s: %s", f.Severity, f.Pkg, f.Message)
		if f.Location != "" {
			line += " (" + f.Location + ")"
		}
		if f.Severity == aurreview.Info {
			output.Info("  " + line)
		} else {
			output.Warning("  " + line)
		}
	}
}
