package brew

import (
	"encoding/json"
	"strings"

	"github.com/jyablonski/arc/internal/shell"
)

type PackageInfo struct {
	Formulae []NamedPackage `json:"formulae"`
	Casks    []NamedPackage `json:"casks"`
}

type NamedPackage struct {
	Name string `json:"name"`
}

func CheckAvailable() error {
	if !shell.CommandExists("brew") {
		return shell.NewErrToolNotAvailable("brew")
	}
	return nil
}

func ListFormulae() ([]string, error) {
	out, err := shell.Run("brew", "list", "--formula")
	if err != nil {
		return nil, err
	}
	return lines(out), nil
}

func ListCasks() ([]string, error) {
	out, err := shell.Run("brew", "list", "--cask")
	if err != nil {
		return nil, err
	}
	return lines(out), nil
}

func Leaves() ([]string, error) {
	out, err := shell.Run("brew", "leaves")
	if err != nil {
		return nil, err
	}
	return lines(out), nil
}

func CacheSize() (string, error) {
	cacheDir, err := shell.Run("brew", "--cache")
	if err != nil {
		return "", err
	}
	out, err := shell.Run("du", "-sh", strings.TrimSpace(cacheDir))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func InstalledInfo() (PackageInfo, error) {
	out, err := shell.Run("brew", "info", "--json=v2", "--installed")
	if err != nil {
		return PackageInfo{}, err
	}
	var info PackageInfo
	if strings.TrimSpace(out) == "" {
		return info, nil
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return PackageInfo{}, err
	}
	return info, nil
}

func lines(out string) []string {
	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}
	}
	return strings.Split(out, "\n")
}
