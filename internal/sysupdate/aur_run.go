package sysupdate

import (
	"fmt"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/aurreview"
)

func publishedAgo(now time.Time, unixSeconds int64) string {
	if unixSeconds <= 0 {
		return ""
	}
	age := now.Sub(time.Unix(unixSeconds, 0))
	if age < 0 {
		return "published in the future"
	}
	switch {
	case age < time.Hour:
		return fmt.Sprintf("published %dm ago", max(1, int(age.Minutes())))
	case age < 48*time.Hour:
		return fmt.Sprintf("published %dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("published %dd ago", int(age.Hours()/24))
	}
}

func aurResultMismatches(result *aurreview.Result, installed map[string]string) []string {
	if result == nil {
		return nil
	}
	var mismatches []string
	for _, update := range result.Updates {
		got := installed[update.Name]
		if isVCSPackage(update.Name) {
			if got == "" || got == update.InstalledVersion {
				mismatches = append(mismatches, fmt.Sprintf("%s remained at %s", update.Name, displayVersion(got)))
			}
			continue
		}
		if got != update.TargetVersion {
			mismatches = append(mismatches, fmt.Sprintf("%s planned %s, installed %s", update.Name, update.TargetVersion, displayVersion(got)))
		}
	}
	return mismatches
}

func isVCSPackage(name string) bool {
	for _, suffix := range []string{"-bzr", "-cvs", "-darcs", "-git", "-hg", "-svn"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
