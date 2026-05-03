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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestRelease(t *testing.T) {
	t.Run("When API returns valid release, it parses correctly", func(t *testing.T) {
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
			expectedPath := fmt.Sprintf("/repos/%s/%s/releases/latest", githubOwner, githubRepo)
			if r.URL.Path != expectedPath {
				t.Errorf("Unexpected path: %s", r.URL.Path)
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(mockRelease))
		}))
		defer server.Close()

		originalAPI := githubAPI
		githubAPI = server.URL
		defer func() { githubAPI = originalAPI }()

		release, err := getLatestRelease()
		require.NoError(t, err)

		assert.Equal(t, "v0.3.0", release.TagName)
		assert.Len(t, release.Assets, 1)

		expectedAssetName := fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH)
		assert.Equal(t, expectedAssetName, release.Assets[0].Name)
	})

	t.Run("When API returns error status, it returns an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not found", http.StatusNotFound)
		}))
		defer server.Close()

		originalAPI := githubAPI
		githubAPI = server.URL
		defer func() { githubAPI = originalAPI }()

		_, err := getLatestRelease()
		assert.Error(t, err)
	})
}

func TestDownloadAndReplace(t *testing.T) {
	t.Run("When downloading valid binary, it replaces the file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testBinaryPath := filepath.Join(tmpDir, "arc")

		mockBinaryContent := []byte("mock binary content")
		err := os.WriteFile(testBinaryPath, mockBinaryContent, 0755)
		require.NoError(t, err, "Failed to create test binary")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, err := w.Write(mockBinaryContent)
			require.NoError(t, err)
		}))
		defer server.Close()

		err = downloadAndReplace(testBinaryPath, server.URL)
		require.NoError(t, err)

		content, err := os.ReadFile(testBinaryPath)
		require.NoError(t, err, "Failed to read replaced binary")
		assert.Equal(t, mockBinaryContent, content)

		info, err := os.Stat(testBinaryPath)
		require.NoError(t, err, "Failed to stat binary")
		assert.NotZero(t, info.Mode().Perm()&0111, "binary should be executable")
	})
}
