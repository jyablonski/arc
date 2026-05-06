package sysupdate

import "fmt"

// kernelChangeMessage builds a warning for the first kernel package change.
func kernelChangeMessage(before, after map[string]string) (updated bool, warning string) {
	for pkg, versionAfter := range after {
		if versionBefore, exists := before[pkg]; exists {
			if versionBefore != versionAfter {
				return true, fmt.Sprintf("Kernel package %s was updated from %s to %s", pkg, versionBefore, versionAfter)
			}
		} else {
			return true, fmt.Sprintf("New kernel package %s was installed (version %s)", pkg, versionAfter)
		}
	}
	return false, ""
}
