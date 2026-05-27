package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultOwner = "jyablonski"
	DefaultRepo  = "arc"
	defaultAPI   = "https://api.github.com"
)

// resolveExecutablePath is overridden in tests so Upgrade does not replace the running test binary.
var resolveExecutablePath = func() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type Updater struct {
	Owner   string
	Repo    string
	APIBase string
}

func New() *Updater {
	return &Updater{
		Owner:   DefaultOwner,
		Repo:    DefaultRepo,
		APIBase: defaultAPI,
	}
}

func (u *Updater) owner() string {
	if u.Owner != "" {
		return u.Owner
	}
	return DefaultOwner
}

func (u *Updater) repo() string {
	if u.Repo != "" {
		return u.Repo
	}
	return DefaultRepo
}

func (u *Updater) apiBase() string {
	if u.APIBase != "" {
		return strings.TrimRight(u.APIBase, "/")
	}
	return defaultAPI
}

func (u *Updater) Upgrade(w io.Writer, currentVersion string) error {
	if w == nil {
		w = os.Stdout
	}

	if currentVersion == "dev" {
		return nil
	}

	latestRelease, err := u.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if CompareVersions(currentVersion, latestRelease.TagName) >= 0 {
		_, _ = fmt.Fprintf(w, "success: You're on the latest version of arc (%s)\n", latestRelease.TagName)
		return nil
	}

	assetName := fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, asset := range latestRelease.Assets {
		if asset.Name == assetName {
			downloadURL = asset.DownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	execPath, err := resolveExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if err := DownloadAndReplace(execPath, downloadURL); err != nil {
		return fmt.Errorf("failed to update binary: %w", err)
	}

	releaseURL := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", u.owner(), u.repo(), latestRelease.TagName)
	_, _ = fmt.Fprintf(w, "success: Upgraded arc from %s to %s! %s\n", currentVersion, latestRelease.TagName, releaseURL)
	return nil
}

func (u *Updater) GetLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", u.apiBase(), u.owner(), u.repo())

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func CompareVersions(current, latest string) int {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		var currentPart, latestPart int
		if i < len(currentParts) {
			_, _ = fmt.Sscanf(currentParts[i], "%d", &currentPart)
		}
		if i < len(latestParts) {
			_, _ = fmt.Sscanf(latestParts[i], "%d", &latestPart)
		}

		if currentPart < latestPart {
			return -1
		}
		if currentPart > latestPart {
			return 1
		}
	}

	return 0
}

func DownloadAndReplace(execPath, downloadURL string) error {
	tempFile := execPath + ".tmp"

	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	if err := os.Chmod(tempFile, 0755); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	if err := os.Rename(tempFile, execPath); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	removeQuarantineBestEffort(execPath)

	return nil
}

func removeQuarantineBestEffort(execPath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	xattr, err := exec.LookPath("xattr")
	if err != nil {
		return
	}
	_ = exec.Command(xattr, "-d", "com.apple.quarantine", execPath).Run()
}
