package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected int // -1 if current < latest, 0 if equal, 1 if current > latest
	}{
		{
			name:     "current is older",
			current:  "v0.1.0",
			latest:   "v0.2.0",
			expected: -1,
		},
		{
			name:     "current is newer",
			current:  "v0.3.0",
			latest:   "v0.2.0",
			expected: 1,
		},
		{
			name:     "versions are equal",
			current:  "v0.2.0",
			latest:   "v0.2.0",
			expected: 0,
		},
		{
			name:     "current without v prefix",
			current:  "0.1.0",
			latest:   "v0.2.0",
			expected: -1,
		},
		{
			name:     "latest without v prefix",
			current:  "v0.1.0",
			latest:   "0.2.0",
			expected: -1,
		},
		{
			name:     "patch version difference",
			current:  "v0.2.0",
			latest:   "v0.2.1",
			expected: -1,
		},
		{
			name:     "minor version difference",
			current:  "v0.2.0",
			latest:   "v0.3.0",
			expected: -1,
		},
		{
			name:     "major version difference",
			current:  "v0.2.0",
			latest:   "v1.0.0",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.current, tt.latest)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.current, tt.latest, result, tt.expected)
			}
		})
	}
}

func TestGetLatestRelease(t *testing.T) {
	// Create a mock GitHub API server
	mockRelease := Release{
		TagName: "v0.3.0",
		Assets: []Asset{
			{
				Name:        fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH),
				DownloadURL: "https://example.com/arc-linux-amd64",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/repos/%s/%s/releases/latest", githubOwner, githubRepo) {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	// Temporarily override the GitHub API URL
	originalAPI := githubAPI
	githubAPI = server.URL
	defer func() { githubAPI = originalAPI }()

	// Test fetching the release
	release, err := getLatestRelease()
	if err != nil {
		t.Fatalf("getLatestRelease() error = %v", err)
	}

	if release.TagName != "v0.3.0" {
		t.Errorf("getLatestRelease() TagName = %q, want %q", release.TagName, "v0.3.0")
	}

	if len(release.Assets) != 1 {
		t.Errorf("getLatestRelease() Assets length = %d, want %d", len(release.Assets), 1)
	}

	expectedAssetName := fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH)
	if release.Assets[0].Name != expectedAssetName {
		t.Errorf("getLatestRelease() Asset Name = %q, want %q", release.Assets[0].Name, expectedAssetName)
	}
}

func TestGetLatestRelease_ErrorHandling(t *testing.T) {
	// Test with a server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	defer server.Close()

	// Temporarily override the GitHub API URL
	originalAPI := githubAPI
	githubAPI = server.URL
	defer func() { githubAPI = originalAPI }()

	// Test fetching the release should fail
	_, err := getLatestRelease()
	if err == nil {
		t.Error("getLatestRelease() expected error, got nil")
	}
}

func TestDownloadAndReplace(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testBinaryPath := filepath.Join(tmpDir, "arc")

	// Create a mock binary file
	mockBinaryContent := []byte("mock binary content")
	err := os.WriteFile(testBinaryPath, mockBinaryContent, 0755)
	if err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Create a mock HTTP server that serves the binary
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(mockBinaryContent)
	}))
	defer server.Close()

	// Test downloading and replacing
	err = downloadAndReplace(testBinaryPath, server.URL)
	if err != nil {
		t.Fatalf("downloadAndReplace() error = %v", err)
	}

	// Verify the file was replaced
	content, err := os.ReadFile(testBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read replaced binary: %v", err)
	}

	if string(content) != string(mockBinaryContent) {
		t.Errorf("downloadAndReplace() content mismatch")
	}

	// Verify file permissions
	info, err := os.Stat(testBinaryPath)
	if err != nil {
		t.Fatalf("Failed to stat binary: %v", err)
	}

	if info.Mode().Perm()&0111 == 0 {
		t.Error("downloadAndReplace() binary is not executable")
	}
}
