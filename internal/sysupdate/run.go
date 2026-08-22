package sysupdate

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
)

// RunWithDeps runs the pacman (and optional yay / cache) update flow.
func RunWithDeps(deps Deps, opts Options) (runErr error) {
	d := mergeDeps(deps)
	if err := d.CheckPacman(); err != nil {
		return err
	}

	started := d.Now()
	renderer := Renderer{Out: d.Out}
	renderer.RunHeader(started)
	log := newMemoryRunLog()
	if opts.Log {
		persistentLog, err := d.NewLog(started)
		if err != nil {
			return err
		}
		log = persistentLog
	}
	defer func() { runErr = closeRunLog(log, runErr) }()
	log.note("arc update system started")
	reader := bufio.NewReader(d.Stdin)

	renderer.Section("SYNC", "")
	authLogStart := log.command("sudo", "-v")
	if err := d.RunLogged(log.Writer(), true, "sudo", "-v"); err != nil {
		return renderRunFailure(renderer, log, authLogStart, "sudo authentication failed", err)
	}
	versionsBeforeKeyring, versionErr := d.InstalledVersions()
	syncStarted := d.Now()
	keyringArgs := []string{"pacman", "-Sy", "--needed", "--noconfirm", "--noprogressbar", "--color", "never", "archlinux-keyring"}
	keyringLogStart := log.command("sudo", keyringArgs...)
	if err := d.RunLogged(log.Writer(), false, "sudo", keyringArgs...); err != nil {
		return renderRunFailure(renderer, log, keyringLogStart, "keyring update failed", err)
	}
	renderer.ResetLine()
	versionsAfterKeyring, afterVersionErr := d.InstalledVersions()
	renderer.Result("databases", "synchronized", d.Now().Sub(syncStarted))
	keyringDetail := packageVersionResult("archlinux-keyring", versionsBeforeKeyring, versionsAfterKeyring, versionErr, afterVersionErr)
	renderer.Result("archlinux-keyring", keyringDetail, 0)
	renderer.Blank()

	kernelPackagesBefore, err := d.KernelVersions()
	if err != nil {
		renderer.Warning(fmt.Sprintf("failed to get kernel packages before update: %v", err))
		kernelPackagesBefore = make(map[string]string)
	}

	applied, declined, err := runRepoUpdate(d, renderer, log, reader, opts.AssumeYes)
	if err != nil {
		return err
	}
	if declined {
		renderer.Warning("repository databases were synchronized but the upgrade was declined; complete a full system upgrade before installing packages")
		renderer.LogPath(log.path)
		log.note("repository upgrade declined")
		return nil
	}

	kernelPackagesAfter := kernelPackagesBefore
	if applied {
		kernelPackagesAfter, err = d.KernelVersions()
		if err != nil {
			renderer.Warning(fmt.Sprintf("failed to get kernel packages after update: %v", err))
			kernelPackagesAfter = make(map[string]string)
		}
	}

	// Note a kernel change now (so the warning is timely) but defer the reboot
	// prompt to the very end: rebooting here would skip AUR updates and cache
	// cleanup. Reboot last, after all update work is done.
	rebootNeeded, rebootMsg := kernelChangeMessage(kernelPackagesBefore, kernelPackagesAfter)
	if rebootNeeded {
		renderer.Warning(rebootMsg)
	}

	if !opts.SkipAUR {
		renderer.Blank()
		if d.CheckYayAvailable() {
			// Triage before yay builds anything; the baseline is committed only
			// when the installed result matches the reviewed plan, so a takeover
			// rejected at the diffmenu stays flagged on the next run.
			result, runYay := runAURReview(d, renderer)
			if runYay {
				renderer.Info("yay review prompts follow; build details stay in the log")
				yayArgs := []string{"-Syu", "--aur", "--diffmenu", "--editmenu", "--answerdiff", "All", "--noansweredit"}
				yayLogStart := log.command("yay", yayArgs...)
				yayOutput := newAUROutput(log.Writer(), renderer)
				err := d.RunLogged(yayOutput, false, "yay", yayArgs...)
				yayOutput.Finish()
				if err != nil {
					renderer.Warning(fmt.Sprintf("yay update failed: %v", err))
					renderer.FailureTail(log.tailFrom(yayLogStart))
				} else {
					commitAURResult(d, renderer, result)
				}
			} else {
				commitAURResult(d, renderer, result)
			}
		} else {
			renderer.Warning("yay is not available, skipping AUR updates")
		}
	}
	if !opts.SkipCache {
		renderer.Blank()
		cacheStarted := d.Now()
		cacheAuthLogStart := log.command("sudo", "-v")
		if err := d.RunLogged(log.Writer(), true, "sudo", "-v"); err != nil {
			renderer.Warning(fmt.Sprintf("package cache authentication failed: %v", err))
			renderer.FailureTail(log.tailFrom(cacheAuthLogStart))
		} else {
			cacheLogStart := log.command("sudo", "paccache", "-rv")
			stopProgress := renderer.Progress("cleaning package cache…")
			err := d.RunLogged(log.Writer(), false, "sudo", "paccache", "-rv")
			stopProgress()
			if err != nil {
				renderer.Warning(fmt.Sprintf("paccache failed: %v", err))
				renderer.FailureTail(log.tailFrom(cacheLogStart))
			} else {
				renderer.Result("cache", "old package archives cleaned", d.Now().Sub(cacheStarted))
			}
		}
	}

	log.note("arc update system finished")
	renderer.LogPath(log.path)

	if rebootNeeded {
		return promptReboot(reader, d.RunInteractive)
	}
	return nil
}

func commitAURResult(d Deps, renderer Renderer, result *aurreview.Result) {
	if result == nil || d.CommitAUR == nil {
		return
	}
	installed := map[string]string{}
	if len(result.Updates) > 0 {
		var err error
		installed, err = d.ForeignPackages()
		if err != nil {
			renderer.Warning(fmt.Sprintf("AUR result could not be verified; provenance baseline not saved: %v", err))
			return
		}
		if mismatches := aurResultMismatches(result, installed); len(mismatches) > 0 {
			renderer.Warning("AUR result did not match the approved plan; provenance baseline not saved: " + strings.Join(mismatches, "; "))
			return
		}
	}
	if err := d.CommitAUR(result); err != nil {
		renderer.Warning(fmt.Sprintf("AUR provenance baseline not saved: %v", err))
		return
	}
	for _, update := range result.Updates {
		renderer.PackageResult(PackageChange{Name: update.Name, FromVersion: update.InstalledVersion, ToVersion: installed[update.Name]})
	}
}

func packageVersionResult(name string, before, after map[string]string, beforeErr, afterErr error) string {
	if beforeErr != nil || afterErr != nil || after[name] == "" {
		return "status unavailable"
	}
	if before[name] == after[name] {
		return after[name] + " (current)"
	}
	if before[name] == "" {
		return after[name] + " (installed)"
	}
	return before[name] + " → " + after[name]
}

func renderRunFailure(renderer Renderer, log *runLog, logStart int64, message string, err error) error {
	log.note(message + ": " + err.Error())
	renderer.Error(message)
	renderer.FailureTail(log.tailFrom(logStart))
	renderer.LogPath(log.path)
	return fmt.Errorf("%s: %w", message, err)
}

// runAURReview fetches AUR metadata and prints triage findings before yay
// runs. It never blocks the update: any failure is reported and skipped. The
// returned result carries the baseline to commit after the result is verified.
func runAURReview(d Deps, renderer Renderer) (*aurreview.Result, bool) {
	if d.ForeignPackages == nil || d.ReviewAUR == nil {
		return nil, true
	}
	installed, err := d.ForeignPackages()
	if err != nil {
		renderer.Warning(fmt.Sprintf("AUR review unavailable: %v", err))
		return nil, true
	}
	installed, ignored := excludeIgnored(d, renderer, installed)
	if len(installed) == 0 {
		renderer.Section("AUR", aurSummary(0, len(ignored)))
		for _, pkg := range ignored {
			renderer.Warning(fmt.Sprintf("%s %s ignored by IgnorePkg", pkg.Name, pkg.Version))
		}
		if len(ignored) == 0 {
			renderer.Result("packages", "no foreign packages", 0)
		} else {
			renderer.Result("review", "no eligible updates", 0)
		}
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	result, err := d.ReviewAUR(ctx, installed)
	if err != nil {
		renderer.Warning(fmt.Sprintf("AUR review unavailable: %v", err))
		return nil, true
	}
	printAURReview(renderer, result, ignored, d.Now())
	return result, len(result.Updates) > 0
}

// excludeIgnored drops packages matched by pacman.conf IgnorePkg (exact or
// glob) so arc doesn't triage updates yay won't apply. Read failures are
// reported and treated as "nothing ignored" rather than blocking the review.
type ignoredPackage struct {
	Name    string
	Version string
}

func excludeIgnored(d Deps, renderer Renderer, installed map[string]string) (map[string]string, []ignoredPackage) {
	if d.IgnoredPackages == nil {
		return installed, nil
	}
	patterns, err := d.IgnoredPackages()
	if err != nil {
		renderer.Warning(fmt.Sprintf("could not read ignored packages: %v", err))
		return installed, nil
	}
	if len(patterns) == 0 {
		return installed, nil
	}
	out := make(map[string]string, len(installed))
	var ignored []ignoredPackage
	for name, ver := range installed {
		if !matchesAnyPattern(name, patterns) {
			out[name] = ver
		} else {
			ignored = append(ignored, ignoredPackage{Name: name, Version: ver})
		}
	}
	sort.Slice(ignored, func(i, j int) bool { return ignored[i].Name < ignored[j].Name })
	return out, ignored
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

func printAURReview(renderer Renderer, result *aurreview.Result, ignored []ignoredPackage, now time.Time) {
	changes := make([]PackageChange, 0, len(result.Updates))
	for _, update := range result.Updates {
		changes = append(changes, PackageChange{
			Name:        update.Name,
			FromVersion: update.InstalledVersion,
			ToVersion:   update.TargetVersion,
			Note:        publishedAgo(now, update.LastModified),
		})
	}
	if len(changes) == 0 {
		renderer.Section("AUR", aurSummary(0, len(ignored)))
	} else {
		renderer.Section("AUR", aurSummary(len(changes), len(ignored)))
		renderer.Plan(changes)
	}
	for _, pkg := range ignored {
		renderer.Warning(fmt.Sprintf("%s %s ignored by IgnorePkg", pkg.Name, pkg.Version))
	}
	if len(result.Findings) == 0 {
		if len(result.Updates) == 0 {
			detail := "no updates pending"
			if len(ignored) > 0 {
				detail = "no eligible updates"
			}
			renderer.Result("review", detail, 0)
		} else {
			renderer.Result("review", "no notable findings", 0)
		}
		return
	}
	renderer.Info("review findings (most severe first)")
	for _, f := range result.Findings {
		line := fmt.Sprintf("[%s] %s: %s", f.Severity, f.Pkg, f.Message)
		if f.Location != "" {
			line += " (" + f.Location + ")"
		}
		if f.Severity == aurreview.Info {
			renderer.Info(line)
		} else {
			renderer.Warning(line)
		}
	}
}

func aurSummary(updates, ignored int) string {
	var parts []string
	if updates > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", updates, plural(updates, "update", "updates")))
	}
	if ignored > 0 {
		parts = append(parts, fmt.Sprintf("%d ignored", ignored))
	}
	if len(parts) == 0 {
		return "up to date"
	}
	return strings.Join(parts, " · ")
}
