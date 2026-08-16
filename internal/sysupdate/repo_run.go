package sysupdate

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"github.com/jyablonski/arc/internal/output"
)

const maxPlanChanges = 3

func runRepoUpdate(d Deps, renderer Renderer, log *runLog, reader *bufio.Reader, assumeYes bool) (applied, declined bool, err error) {
	planLogStart := log.command("pacman", "-Sup", "--print-format", pacmanPlanFormat)
	plan, err := d.RepoPlan()
	if err != nil {
		return false, false, renderRunFailure(renderer, log, planLogStart, "repository plan failed", err)
	}
	plan = normalizedPlan(plan)
	logRepoPlan(log, plan)
	if len(plan) == 0 {
		renderer.Section("REPO", "up to date")
		renderer.Result("packages", "no updates", 0)
		return false, false, nil
	}

	for attempt := 0; attempt < maxPlanChanges; attempt++ {
		renderer.Section("REPO", fmt.Sprintf("%d %s", len(plan), plural(len(plan), "update", "updates")))
		renderer.Plan(plan)
		if assumeYes {
			renderer.Info("repository approval bypassed by --yes")
		} else {
			renderer.Prompt(fmt.Sprintf("Proceed with %d repo %s?", len(plan), plural(len(plan), "upgrade", "upgrades")))
			approvalLogStart := log.position()
			approved, err := readApproval(reader)
			renderer.Blank()
			if err != nil {
				return false, false, renderRunFailure(renderer, log, approvalLogStart, "repository approval failed", err)
			}
			if !approved {
				renderer.Info("repository upgrade skipped")
				return false, true, nil
			}
		}

		confirmedLogStart := log.command("pacman", "-Sup", "--print-format", pacmanPlanFormat)
		confirmed, err := d.RepoPlan()
		if err != nil {
			return false, false, renderRunFailure(renderer, log, confirmedLogStart, "repository plan revalidation failed", err)
		}
		confirmed = normalizedPlan(confirmed)
		logRepoPlan(log, confirmed)
		if len(confirmed) == 0 {
			renderer.Warning("repository plan changed while awaiting approval; no updates remain")
			renderer.Blank()
			renderer.Section("REPO", "up to date")
			renderer.Result("packages", "no updates", 0)
			return false, false, nil
		}
		if samePackagePlan(plan, confirmed) {
			plan = confirmed
			break
		}
		if attempt == maxPlanChanges-1 {
			return false, false, renderRunFailure(renderer, log, confirmedLogStart, "repository plan remained unstable",
				fmt.Errorf("plan changed %d times; rerun the update when mirrors and package state are stable", maxPlanChanges))
		}
		renderer.Warning("repository plan changed while awaiting approval; review the updated transaction")
		renderer.Blank()
		plan = confirmed
	}

	started := d.Now()
	args := []string{"pacman", "-Su", "--noconfirm", "--noprogressbar", "--color", "never"}
	updateLogStart := log.command("sudo", args...)
	stopProgress := renderer.Progress(fmt.Sprintf("updating %d %s…", len(plan), plural(len(plan), "package", "packages")))
	err = d.RunLogged(log.Writer(), false, "sudo", args...)
	stopProgress()
	if err != nil {
		return false, false, renderRunFailure(renderer, log, updateLogStart, "pacman update failed", err)
	}

	installed, err := d.InstalledVersions()
	if err != nil {
		return false, false, renderRunFailure(renderer, log, updateLogStart, "verify installed package versions failed", err)
	}
	for _, change := range plan {
		if installed[change.Name] != change.ToVersion {
			return false, false, renderRunFailure(renderer, log, updateLogStart, "pacman result verification failed",
				fmt.Errorf("%s: planned %s, installed %s", change.Name, change.ToVersion, displayVersion(installed[change.Name])))
		}
		for _, replaced := range strings.Split(change.Replaces, ",") {
			if replaced != "" && installed[replaced] != "" {
				return false, false, renderRunFailure(renderer, log, updateLogStart, "pacman result verification failed",
					fmt.Errorf("%s was meant to replace %s, but %s remains installed", change.Name, replaced, replaced))
			}
		}
	}

	detail := fmt.Sprintf("%d %s", len(plan), plural(len(plan), "package", "packages"))
	if size := totalDownloadSize(plan); size > 0 {
		detail += ", " + output.Bytes(size)
	}
	renderer.Result("transaction", detail, d.Now().Sub(started))
	renderer.Result("verified", "installed package state", 0)
	for _, change := range plan {
		renderer.PackageResult(change)
	}
	renderer.Result("hooks", "post-transaction complete", 0)
	return true, false, nil
}

func logRepoPlan(log *runLog, plan []PackageChange) {
	if len(plan) == 0 {
		log.note("repository plan: no updates")
		return
	}
	for _, change := range normalizedPlan(plan) {
		log.note(fmt.Sprintf("repository plan: %s %s -> %s (%d bytes, replaces %s)", change.Name, displayVersion(change.FromVersion), change.ToVersion, change.SizeBytes, displayVersion(change.Replaces)))
	}
}

func readApproval(reader *bufio.Reader) (bool, error) {
	response, err := reader.ReadString('\n')
	if err != nil && response == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(response)) {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

func normalizedPlan(plan []PackageChange) []PackageChange {
	plan = append([]PackageChange(nil), plan...)
	sort.Slice(plan, func(i, j int) bool { return plan[i].Name < plan[j].Name })
	return plan
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
