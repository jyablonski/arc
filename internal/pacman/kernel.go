package pacman

import "strings"

var kernelLinePrefixes = []string{
	"linux ", "linux-lts ", "linux-zen ", "linux-hardened ",
	"linux-headers ", "linux-lts-headers ", "linux-zen-headers ", "linux-hardened-headers ",
}

func InstalledKernelVersions() (map[string]string, error) {
	out, err := run.Run("pacman", "-Q")
	if err != nil {
		return nil, err
	}
	return ParseInstalledKernelPackages(out), nil
}

func ParseInstalledKernelPackages(pacmanQ string) map[string]string {
	kernelPkgs := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(pacmanQ), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		for _, prefix := range kernelLinePrefixes {
			if strings.HasPrefix(line, prefix) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					pkgName := parts[0]
					version := parts[1]
					if !strings.Contains(pkgName, "-headers") {
						kernelPkgs[pkgName] = version
					}
				}
				break
			}
		}
	}
	return kernelPkgs
}
