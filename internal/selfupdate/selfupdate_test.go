package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected int
	}{
		{name: "current is older", current: "v0.1.0", latest: "v0.2.0", expected: -1},
		{name: "current is newer", current: "v0.3.0", latest: "v0.2.0", expected: 1},
		{name: "versions are equal", current: "v0.2.0", latest: "v0.2.0", expected: 0},
		{name: "current without v prefix", current: "0.1.0", latest: "v0.2.0", expected: -1},
		{name: "latest without v prefix", current: "v0.1.0", latest: "0.2.0", expected: -1},
		{name: "patch version difference", current: "v0.2.0", latest: "v0.2.1", expected: -1},
		{name: "minor version difference", current: "v0.2.0", latest: "v0.3.0", expected: -1},
		{name: "major version difference", current: "v0.2.0", latest: "v1.0.0", expected: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CompareVersions(tt.current, tt.latest))
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
			expectedPath := fmt.Sprintf("/repos/%s/%s/releases/latest", DefaultOwner, DefaultRepo)
			if r.URL.Path != expectedPath {
				t.Errorf("Unexpected path: %s", r.URL.Path)
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(mockRelease))
		}))
		defer server.Close()

		u := &Updater{Owner: DefaultOwner, Repo: DefaultRepo, APIBase: server.URL}
		release, err := u.GetLatestRelease()
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

		u := &Updater{Owner: DefaultOwner, Repo: DefaultRepo, APIBase: server.URL}
		_, err := u.GetLatestRelease()
		assert.Error(t, err)
	})

	t.Run("When API returns invalid JSON, it returns an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer server.Close()

		u := &Updater{Owner: DefaultOwner, Repo: DefaultRepo, APIBase: server.URL}
		_, err := u.GetLatestRelease()
		require.Error(t, err)
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

		err = DownloadAndReplace(testBinaryPath, server.URL)
		require.NoError(t, err)

		content, err := os.ReadFile(testBinaryPath)
		require.NoError(t, err, "Failed to read replaced binary")
		assert.Equal(t, mockBinaryContent, content)

		info, err := os.Stat(testBinaryPath)
		require.NoError(t, err, "Failed to stat binary")
		assert.NotZero(t, info.Mode().Perm()&0111, "binary should be executable")
	})

	t.Run("When download returns non-200, it errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "arc")
		require.NoError(t, os.WriteFile(path, []byte("old"), 0o755))

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		err := DownloadAndReplace(path, srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "download failed with status 404")
	})
}

func TestUpgrade_devVersionIsNoop(t *testing.T) {
	var b strings.Builder
	require.NoError(t, New().Upgrade(&b, "dev"))
	assert.Empty(t, b.String())
}

func TestUpgrade_alreadyLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rel := Release{
			TagName: "v1.0.0",
			Assets: []Asset{{
				Name:        fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH),
				DownloadURL: "https://example.invalid/asset",
			}},
		}
		require.NoError(t, json.NewEncoder(w).Encode(rel))
	}))
	defer server.Close()

	var buf strings.Builder
	u := New()
	u.APIBase = server.URL
	require.NoError(t, u.Upgrade(&buf, "v1.0.0"))
	require.Contains(t, buf.String(), "latest version of arc")
}

func TestUpgrade_missingReleaseAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rel := Release{
			TagName: "v9.0.0",
			Assets:  []Asset{{Name: "wrong-asset-name.tar.gz", DownloadURL: "http://localhost/x"}},
		}
		require.NoError(t, json.NewEncoder(w).Encode(rel))
	}))
	defer server.Close()

	u := New()
	u.APIBase = server.URL
	err := u.Upgrade(io.Discard, "v0.1.0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no release asset found")
}

func TestUpgrade_apiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := New()
	u.APIBase = server.URL
	err := u.Upgrade(io.Discard, "v0.1.0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get latest release")
}

func TestUpgrade_downloadsNewVersion(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "arc")
	require.NoError(t, os.WriteFile(binPath, []byte("old"), 0o755))

	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, err := w.Write([]byte("newbinary"))
		require.NoError(t, err)
	}))
	defer dl.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rel := Release{
			TagName: "v2.0.0",
			Assets: []Asset{{
				Name:        fmt.Sprintf("arc-%s-%s", runtime.GOOS, runtime.GOARCH),
				DownloadURL: dl.URL,
			}},
		}
		require.NoError(t, json.NewEncoder(w).Encode(rel))
	}))
	defer api.Close()

	prev := resolveExecutablePath
	t.Cleanup(func() { resolveExecutablePath = prev })
	resolveExecutablePath = func() (string, error) { return binPath, nil }

	var buf strings.Builder
	u := New()
	u.APIBase = api.URL
	require.NoError(t, u.Upgrade(&buf, "v0.1.0"))
	require.Contains(t, buf.String(), "Upgraded arc")

	got, err := os.ReadFile(binPath)
	require.NoError(t, err)
	require.Equal(t, []byte("newbinary"), got)
}
