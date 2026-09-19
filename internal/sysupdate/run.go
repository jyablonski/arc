package sysupdate

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/jyablonski/arc/internal/output"
)

// RunWithDeps runs the pacman (and optional yay / cache) update flow.
func RunWithDeps(deps Deps, opts Options) (runErr error) {
	d := mergeDeps(deps)
	if err := d.CheckPacman(); err != nil {
		return err
	}

	log := newMemoryRunLog()
	// Acquire sudo before anything is drawn so the password prompt (in sudo's
	// own wording) never lands inside a section.
	authLogStart := log.command("sudo", "-v")
	if err := d.RunLogged(log.Writer(), true, "sudo", "-v"); err != nil {
		renderer := NewRenderer(d.Out, opts.Verbose)
		return renderRunFailure(renderer, log, authLogStart, "sudo authentication failed", err)
	}

	started := d.Now()
	renderer := NewRenderer(d.Out, opts.Verbose)
	renderer.RunHeader(started)

	var footerNotes []string
	footerDone := false
	// Registered before the log-closing defer so it runs after it: every exit
	// past the header closes with the summary line, and that line reports a
	// failure to close the log too.
	defer func() {
		if footerDone {
			return
		}
		if runErr != nil {
			footerNotes = append(footerNotes, renderer.style().Red("failed"))
		}
		renderer.Footer(d.Now().Sub(started), footerNotes...)
	}()
	defer func() { runErr = closeRunLog(log, runErr) }()

	if opts.Log {
		persistentLog, err := d.NewLog(started)
		if err != nil {
			return err
		}
		log = persistentLog
	}
	log.note("arc update system started")
	reader := bufio.NewReader(d.Stdin)

	versionsBeforeKeyring, versionErr := d.InstalledVersions()
	syncStarted := d.Now()
	keyringArgs := []string{"pacman", "-Sy", "--needed", "--noconfirm", "--noprogressbar", "--color", "never", "archlinux-keyring"}
	keyringLogStart := log.command("sudo", keyringArgs...)
	stopProgress := renderer.Progress("synchronizing package databases…")
	err := d.RunLogged(log.Writer(), false, "sudo", keyringArgs...)
	stopProgress()
	if err != nil {
		renderer.Section("SYNC", "")
		return renderRunFailure(renderer, log, keyringLogStart, "keyring update failed", err)
	}
	renderer.ResetLine()
	versionsAfterKeyring, afterVersionErr := d.InstalledVersions()
	renderer.Section("SYNC", "")
	renderer.Result("databases", "synchronized", d.Now().Sub(syncStarted))
	keyring, changed := packageVersionResult("archlinux-keyring", versionsBeforeKeyring, versionsAfterKeyring, versionErr, afterVersionErr)
	if changed {
		renderer.Result("archlinux-keyring", keyring, 0)
		renderer.countUpgraded(1)
	} else {
		renderer.InfoResult("archlinux-keyring", keyring)
	}
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
		footerNotes = append(footerNotes, "repo upgrade declined")
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
			if note := runAURUpdate(d, renderer, log, reader, opts); note != "" {
				footerNotes = append(footerNotes, note)
			}
		} else {
			renderer.Section("AUR", "")
			renderer.Warning("yay is not available, skipping AUR updates")
		}
	}
	if !opts.SkipCache {
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
				renderer.Result("cache", "old archives cleaned", d.Now().Sub(cacheStarted))
			}
		}
	}

	log.note("arc update system finished")
	renderer.LogPath(log.path)

	if rebootNeeded {
		// The summary belongs above the reboot question, which ends the run.
		renderer.Footer(d.Now().Sub(started), footerNotes...)
		footerDone = true
		return promptReboot(renderer, reader, d.RunInteractive)
	}
	return nil
}

// runAURUpdate reviews pending AUR updates, asks once in arc's wording, and
// runs yay behind the stdin proxy. It returns a footer note for outcomes
// the summary line should mention.
func runAURUpdate(d Deps, renderer Renderer, log *runLog, reader *bufio.Reader, opts Options) string {
	// Triage before yay builds anything; the baseline is committed only when
	// the installed result matches the reviewed plan, so a takeover rejected
	// at the gate stays flagged on the next run.
	result, runYay := runAURReview(d, renderer, opts.ShowDiff)
	if !runYay {
		commitAURResult(d, renderer, result, nil)
		return ""
	}

	var approved map[string]bool
	if result != nil {
		ok, err := approveAUR(renderer, reader, result)
		if err != nil {
			renderer.Warning(fmt.Sprintf("AUR approval failed: %v", err))
			return "AUR skipped"
		}
		if !ok {
			renderer.Info("AUR upgrade skipped")
			log.note("AUR upgrade declined")
			return "AUR declined"
		}
		approved = make(map[string]bool, len(result.Updates))
		for _, u := range result.Updates {
			approved[u.Name] = true
		}
	}

	yayArgs := []string{
		"-Syu", "--aur",
		"--answerupgrade", "None",
		"--cleanmenu=false",
		"--editmenu=false",
	}
	if result == nil {
		// arc could not review, so yay shows its own build-file diffs.
		yayArgs = append(yayArgs, "--diffmenu", "--answerdiff", "All")
	} else {
		yayArgs = append(yayArgs, "--diffmenu=false")
	}

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		renderer.Warning(fmt.Sprintf("yay update failed: %v", err))
		return ""
	}
	yayLogStart := log.command("yay", yayArgs...)
	yayOutput := newAUROutput(log.Writer(), renderer, aurOutputOptions{
		diffPackages: aurPackageBases(result),
		answers:      stdinW,
		input:        reader,
		approved:     approved,
		now:          d.Now,
	})
	err = d.RunAUR(yayOutput, stdinR, "yay", yayArgs...)
	_ = stdinW.Close()
	_ = stdinR.Close()
	yayOutput.Finish()
	renderer.ResetLine()
	if err != nil {
		renderer.Warning(fmt.Sprintf("yay update failed: %v", err))
		renderer.FailureTail(log.tailFrom(yayLogStart))
		return ""
	}
	commitAURResult(d, renderer, result, yayOutput.BuildTimes())
	// pacman reports the net size only once the built packages exist, so it
	// lands here rather than in the section summary or the prompt.
	if net := yayOutput.NetBytes(); net > 0 {
		renderer.InfoResult("disk", output.Bytes(net)+" net")
	}
	return ""
}

// approveAUR is the single AUR gate. Enter means yes unless the review found
// a high-signal problem, in which case Enter means no. "d" prints every diff
// and asks again.
func approveAUR(renderer Renderer, reader *bufio.Reader, result *aurreview.Result) (bool, error) {
	defaultYes := true
	for _, f := range result.Findings {
		if f.Severity == aurreview.High {
			defaultYes = false
		}
	}
	label := fmt.Sprintf("Upgrade %d AUR %s?", len(result.Updates), plural(len(result.Updates), "package", "packages"))
	for {
		renderer.Prompt(label, defaultYes)
		response, err := reader.ReadString('\n')
		renderer.EndPrompt()
		if err != nil && response == "" {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(response)) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "d", "diff":
			if !printAURDiffs(renderer, result, func(aurreview.Change) bool { return true }) {
				renderer.Info("no build-file diffs available; arc has no trusted snapshot to compare against")
			}
			renderer.Blank()
		default:
			return false, nil
		}
	}
}

func commitAURResult(d Deps, renderer Renderer, result *aurreview.Result, builds map[string]time.Duration) {
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
		renderer.PackageResult(PackageChange{Name: update.Name, FromVersion: update.InstalledVersion, ToVersion: installed[update.Name]}, builds[packageBase(update)])
	}
	renderer.countUpgraded(len(result.Updates))
}

func packageBase(u aurreview.Update) string {
	if u.PackageBase != "" {
		return u.PackageBase
	}
	return u.Name
}

// packageVersionResult describes a package after sync and whether it changed.
func packageVersionResult(name string, before, after map[string]string, beforeErr, afterErr error) (string, bool) {
	if beforeErr != nil || afterErr != nil || after[name] == "" {
		return "status unavailable", false
	}
	if before[name] == after[name] {
		return after[name] + "  current", false
	}
	if before[name] == "" {
		return after[name] + "  installed", true
	}
	return before[name] + " → " + after[name], true
}

func renderRunFailure(renderer Renderer, log *runLog, logStart int64, message string, err error) error {
	log.note(message + ": " + err.Error())
	renderer.Error(message)
	renderer.FailureTail(log.tailFrom(logStart))
	renderer.LogPath(log.path)
	return fmt.Errorf("%s: %w", message, err)
}

// runAURReview fetches AUR metadata and prints the AUR section: the plan with
// each package's change classification, held packages, and the review verdict.
// It never blocks the update: a failed review falls back to yay's own diffs.
// The returned result carries the baseline to commit after verification.
func runAURReview(d Deps, renderer Renderer, showDiff bool) (*aurreview.Result, bool) {
	if d.ForeignPackages == nil || d.ReviewAUR == nil {
		renderer.Section("AUR", "")
		return nil, true
	}
	installed, err := d.ForeignPackages()
	if err != nil {
		renderer.Section("AUR", "")
		renderer.Warning(fmt.Sprintf("AUR review unavailable: %v", err))
		return nil, true
	}
	installed, ignored, ignoreErr := excludeIgnored(d, installed)
	renderer.countIgnored(len(ignored))
	if len(installed) == 0 {
		renderer.Section("AUR", aurSummary(0, len(ignored), renderer.style().Sep()))
		renderer.Plan(nil, ignored...)
		if ignoreErr != nil {
			renderer.Warning(ignoreErr.Error())
		}
		if len(ignored) == 0 {
			renderer.InfoResult("packages", "no foreign packages")
		} else {
			renderer.InfoResult("review", "no eligible updates")
		}
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	stopProgress := renderer.Progress("reviewing AUR packages…")
	result, err := d.ReviewAUR(ctx, installed)
	stopProgress()
	if err != nil {
		renderer.Section("AUR", aurSummary(0, len(ignored), renderer.style().Sep()))
		renderer.Plan(nil, ignored...)
		renderer.Warning(fmt.Sprintf("AUR review unavailable, yay will show its own diffs: %v", err))
		return nil, true
	}
	if ignoreErr != nil {
		renderer.Warning(ignoreErr.Error())
	}
	printAURReview(renderer, result, ignored, d.Now(), showDiff)
	return result, len(result.Updates) > 0
}

// excludeIgnored drops packages matched by pacman.conf IgnorePkg (exact or
// glob) so arc doesn't triage updates yay won't apply. Read failures are
// reported and treated as "nothing ignored" rather than blocking the review.
type ignoredPackage struct {
	Name    string
	Version string
}

func excludeIgnored(d Deps, installed map[string]string) (map[string]string, []ignoredPackage, error) {
	if d.IgnoredPackages == nil {
		return installed, nil, nil
	}
	patterns, err := d.IgnoredPackages()
	if err != nil {
		return installed, nil, fmt.Errorf("could not read ignored packages: %w", err)
	}
	if len(patterns) == 0 {
		return installed, nil, nil
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
	return out, ignored, nil
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

func printAURReview(renderer Renderer, result *aurreview.Result, ignored []ignoredPackage, now time.Time, showDiff bool) {
	changes := make([]PackageChange, 0, len(result.Updates))
	for _, update := range result.Updates {
		c := PackageChange{
			Name:        update.Name,
			FromVersion: update.InstalledVersion,
			ToVersion:   update.TargetVersion,
			Note:        publishedAgo(now, update.LastModified),
		}
		change, ok := result.Changes[packageBase(update)]
		switch {
		case ok:
			c.Change = change.Summary
			c.Attention = !change.Routine
		default:
			c.Change = "unchanged since last review"
		}
		changes = append(changes, c)
	}
	renderer.Section("AUR", aurSummary(len(changes), len(ignored), renderer.style().Sep()))
	renderer.Plan(changes, ignored...)
	if len(changes) == 0 {
		detail := "no updates pending"
		if len(ignored) > 0 {
			detail = "no eligible updates"
		}
		renderer.InfoResult("review", detail)
		return
	}

	// Diffs for build-logic changes expand by default; routine bumps stay
	// behind "d" at the prompt or --diff.
	expand := func(c aurreview.Change) bool { return showDiff || !c.Routine }
	shownDiffs := printAURDiffs(renderer, result, expand)

	renderer.Blank()
	attention := 0
	for _, c := range changes {
		if c.Attention {
			attention++
		}
	}
	actionable := make([]aurreview.Finding, 0, len(result.Findings))
	for _, finding := range result.Findings {
		if finding.Severity >= aurreview.Warn {
			actionable = append(actionable, finding)
		}
	}
	hint := renderer.style().Faint("d to diff")
	switch {
	case len(actionable) > 0:
		renderer.Warning(fmt.Sprintf("%d suspicious %s detected", len(actionable), plural(len(actionable), "change", "changes")))
		for _, f := range actionable {
			line := fmt.Sprintf("%s: %s", f.Pkg, f.Message)
			if f.Location != "" {
				line += " (" + f.Location + ")"
			}
			if f.Severity == aurreview.High {
				renderer.Error(line)
			} else {
				renderer.Warning(line)
			}
		}
	case attention > 0 && shownDiffs:
		renderer.writeStatusHint(output.GlyphWarn, "review", fmt.Sprintf("%s changed build logic, diff above", output.Count(attention, "package", "packages")), hint)
	case attention > 0:
		renderer.writeStatusHint(output.GlyphWarn, "review", fmt.Sprintf("%s could not be compared", output.Count(attention, "package", "packages")), hint)
	default:
		renderer.writeStatusHint(output.GlyphOK, "review", "no build logic changed", hint)
	}
}

// printAURDiffs renders arc's own diffs (against the last trusted snapshot)
// for every reviewed package base whose change passes include. It reports
// whether anything was printed.
func printAURDiffs(renderer Renderer, result *aurreview.Result, include func(aurreview.Change) bool) bool {
	printed := false
	for _, base := range slices.Sorted(maps.Keys(result.Changes)) {
		change := result.Changes[base]
		if len(change.Files) == 0 || !include(change) {
			continue
		}
		printed = true
		renderer.DiffPackage(base, change.Summary)
		for _, f := range change.Files {
			renderer.DiffFile(f.Name)
			for _, line := range f.Lines {
				renderer.DiffLine(line)
			}
		}
	}
	return printed
}

func aurPackageBases(result *aurreview.Result) []string {
	if result == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(result.Updates))
	for _, update := range result.Updates {
		base := update.PackageBase
		if base == "" {
			base = update.Name
		}
		seen[base] = struct{}{}
	}
	bases := make([]string, 0, len(seen))
	for base := range seen {
		bases = append(bases, base)
	}
	sort.Strings(bases)
	return bases
}

func aurSummary(updates, ignored int, sep string) string {
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
	return strings.Join(parts, sep)
}
