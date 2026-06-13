package aurreview_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/aurreview"
	"github.com/jyablonski/arc/internal/boundary"
	"github.com/stretchr/testify/require"
)

type pkgMeta struct {
	base       string
	version    string
	maintainer *string // nil = orphaned
	outOfDate  *int64
	pkgbuild   string
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

// router fakes the AUR: the RPC info endpoint emits one result per requested
// arg[], and cgit plain returns the package's PKGBUILD. scanned (if non-nil)
// records every pkgbase whose files were fetched, so tests can assert gating.
func router(meta map[string]pkgMeta, scanned *[]string) func(*http.Request) (*http.Response, error) {
	baseOf := func(name string, m pkgMeta) string {
		if m.base != "" {
			return m.base
		}
		return name
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
					`{"Name":%q,"PackageBase":%q,"Version":%q,"Maintainer":%s,"OutOfDate":%s}`,
					n, baseOf(n, m), m.version, maint, ood))
			}
			body := fmt.Sprintf(`{"type":"multiinfo","resultcount":%d,"results":[%s]}`,
				len(results), strings.Join(results, ","))
			return jsonResp(http.StatusOK, body), nil
		}
		// cgit plain file fetch.
		base := u.Query().Get("h")
		if scanned != nil {
			*scanned = append(*scanned, base)
		}
		for n, m := range meta {
			if baseOf(n, m) == base {
				return jsonResp(http.StatusOK, m.pkgbuild), nil
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
		"foo": {version: "2.0", maintainer: new("alice"), pkgbuild: "pkgname=foo\nbuild() { npm install }\n"},
	}
	rv := newReviewer(statePath, router(meta, nil))

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

	// Commit writes it (creating the missing dir).
	require.NoError(t, rv.Commit(res))
	raw, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"maintainer": "alice"`)
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
	rv := newReviewer(statePath, router(meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.High, `"alice" -> "mallory"`))
	// Most-severe-first ordering.
	require.Equal(t, aurreview.High, res.Findings[0].Severity)
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
	rv := newReviewer(statePath, router(meta, nil))

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
		"bumped": {version: "2.0", maintainer: new("bob"), pkgbuild: "pkgname=bumped\n"},
	}
	var scanned []string
	rv := newReviewer(statePath, router(meta, &scanned))

	_, err := rv.Review(context.Background(), map[string]string{"stable": "1.0", "bumped": "1.0"})
	require.NoError(t, err)
	require.Contains(t, scanned, "bumped")
	require.NotContains(t, scanned, "stable")
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
		"spotify": {version: "2.0", maintainer: new("alice"), pkgbuild: pkgbuild},
	}
	rv := newReviewer(statePath, router(meta, nil))

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

func TestReview_orphanAndOutOfDate(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	ood := int64(1700000000)
	meta := map[string]pkgMeta{
		"foo": {version: "1.0", maintainer: nil, outOfDate: &ood, pkgbuild: "pkgname=foo\n"},
	}
	rv := newReviewer(statePath, router(meta, nil))

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "ORPHANED"))
	require.True(t, findingFor(res.Findings, "foo", aurreview.Info, "flagged out-of-date"))
}

func TestReview_pkgbuildFetchFailsYieldsWarning(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	do := func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/rpc/") {
			return jsonResp(http.StatusOK,
				`{"type":"multiinfo","resultcount":1,"results":[{"Name":"foo","PackageBase":"foo","Version":"2.0","Maintainer":"alice"}]}`), nil
		}
		return jsonResp(http.StatusServiceUnavailable, "down"), nil
	}
	rv := newReviewer(statePath, do)

	res, err := rv.Review(context.Background(), map[string]string{"foo": "1.0"})
	require.NoError(t, err)
	require.True(t, findingFor(res.Findings, "foo", aurreview.Warn, "could not fetch PKGBUILD"))
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
