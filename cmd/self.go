package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	githubOwner = "jyablonski"
	githubRepo  = "arc"
)

var (
	githubAPI = "https://api.github.com"
)

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage arc itself",
	Long:  `Commands for managing the arc binary itself.`,
}

var selfUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update arc to the latest version",
	Long:  `Check for the latest release on GitHub and update arc to that version if available.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current version
		currentVersion := version
		if currentVersion == "dev" {
			return nil
		}

		// Get latest release
		latestRelease, err := getLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to get latest release: %w", err)
		}

		// Compare versions
		if compareVersions(currentVersion, latestRelease.TagName) >= 0 {
			fmt.Printf("success: You're on the latest version of arc (%s)\n", latestRelease.TagName)
			return nil
		}

		// Find the correct asset for this platform
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

		// Get current executable path
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// Resolve symlinks to get the actual path
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("failed to resolve executable path: %w", err)
		}

		// Download and replace binary
		if err := downloadAndReplace(execPath, downloadURL); err != nil {
			return fmt.Errorf("failed to update binary: %w", err)
		}

		releaseURL := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", githubOwner, githubRepo, latestRelease.TagName)
		fmt.Printf("success: Upgraded arc from %s to %s! %s\n", currentVersion, latestRelease.TagName, releaseURL)
		return nil
	},
}

func getLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPI, githubOwner, githubRepo)

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

func compareVersions(current, latest string) int {
	// Remove 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Simple comparison: split by dots and compare numerically
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

func downloadAndReplace(execPath, downloadURL string) error {
	// Download to temporary file
	tempFile := execPath + ".tmp"

	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	out, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	// Copy response body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	// Make executable
	if err := os.Chmod(tempFile, 0755); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	// Replace the original binary
	if err := os.Rename(tempFile, execPath); err != nil {
		_ = os.Remove(tempFile)
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(selfCmd)
	selfCmd.AddCommand(selfUpdateCmd)
}
