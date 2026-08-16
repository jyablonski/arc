// Package aurreview adds a triage layer on top of yay's interactive diffmenu:
//  1. provenance baseline tracking: maintainer changes, orphan adoptions, and
//     packages deleted out from under you are all takeover signals
//  2. static scan of the incoming package files — the full cgit snapshot
//     (PKGBUILD, .install, hooks, patches, local sources), diffed against the
//     last trusted snapshot so only lines you haven't already vetted are
//     flagged
//  3. cross-package cluster detection (one account touching many at once)
//
// It NEVER decides for you. It surfaces findings and routes your attention;
// you still make the call at the diffmenu. grep-based scanning has false
// positives (e.g. a SKIP sum that's overridden on the next line) and is
// defeated by obfuscation, so treat HIGH findings as "look here first",
// not "reject".
//
// Review computes findings without touching the baseline; Commit persists the
// observed maintainers/versions plus the file snapshots after the planned
// updates are installed or the review confirms that none are pending. That
// ordering keeps a declined or unsuccessful update from advancing trust.
package aurreview

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jyablonski/arc/internal/boundary"
	"github.com/jyablonski/arc/internal/statepath"
)

const (
	rpcInfoURL      = "https://aur.archlinux.org/rpc/v5/info"
	cgitPlainURL    = "https://aur.archlinux.org/cgit/aur.git/plain"
	cgitSnapshotURL = "https://aur.archlinux.org/cgit/aur.git/snapshot"
	rpcBatchLimit   = 150 // official instance caps very large info requests
	userAgent       = "arc-aur-review (+https://github.com/jyablonski/arc)"

	maxFileBytes     = 1 << 20 // per-file cap; larger files are reported, not scanned
	maxSnapshotBytes = 8 << 20 // snapshot tarball cap
	scanWorkers      = 4       // parallel snapshot fetches; keep it polite to the AUR
)

// ---- RPC types (subset of v5 info result) ----

type rpcInfo struct {
	Name           string  `json:"Name"`
	PackageBase    string  `json:"PackageBase"`
	Version        string  `json:"Version"`
	Maintainer     *string `json:"Maintainer"` // null when orphaned
	OutOfDate      *int64  `json:"OutOfDate"`  // unix ts when flagged, else null
	FirstSubmitted int64   `json:"FirstSubmitted"`
	LastModified   int64   `json:"LastModified"`
	NumVotes       int     `json:"NumVotes"`
	Popularity     float64 `json:"Popularity"`
}

type rpcResponse struct {
	Type        string    `json:"type"`
	ResultCount int       `json:"resultcount"`
	Results     []rpcInfo `json:"results"`
	Error       string    `json:"error"`
}

// ---- Findings ----

// Severity orders findings from informational to high-signal.
type Severity int

const (
	Info Severity = iota
	Warn
	High
)

func (s Severity) String() string {
	switch s {
	case High:
		return "HIGH"
	case Warn:
		return "WARN"
	default:
		return "INFO"
	}
}

// Finding is one surfaced observation about a pending AUR change.
type Finding struct {
	Pkg      string
	Severity Severity
	Message  string
	Location string // "PKGBUILD:14" when from a file scan, else ""
}

// ---- Provenance state ----

type provenance struct {
	Maintainer   string `json:"maintainer"`
	Version      string `json:"version"`
	SeenAt       int64  `json:"seen_at"`
	LastModified int64  `json:"last_modified,omitempty"` // AUR-side push time at last trusted run
}

type state struct {
	path string
	data map[string]provenance
}

func loadState(path string) (*state, error) {
	s := &state{path: path, data: map[string]provenance{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil // first run; empty baseline
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("corrupt state %s: %w", path, err)
		}
	}
	return s, nil
}

func saveState(path string, data map[string]provenance) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path) // atomic-ish replace
}

// DefaultStatePath returns the baseline location, honoring XDG_STATE_HOME and
// falling back to ~/.local/state.
func DefaultStatePath() (string, error) {
	dir, err := statepath.ArcDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "aur-provenance.json"), nil
}

// ---- Reviewer ----

// Reviewer fetches AUR metadata and produces triage findings.
type Reviewer struct {
	HTTP      boundary.HTTPDoer
	StatePath string
	// CacheDir holds the file snapshots from the last trusted (committed) run,
	// one subdirectory per pkgbase. Scans diff against it so only new lines are
	// flagged. Empty disables diffing (every scan is a full scan).
	CacheDir string
}

// New returns a Reviewer with a 15s HTTP client writing its baseline and
// snapshot cache next to statePath.
func New(statePath string) *Reviewer {
	return &Reviewer{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		StatePath: statePath,
		CacheDir:  filepath.Join(filepath.Dir(statePath), "aur-files"),
	}
}

// Result carries the findings, pending updates, and baseline that Commit should
// persist after the reviewed installed state is trusted.
type Result struct {
	Findings []Finding
	Updates  []Update
	baseline map[string]provenance
	files    map[string]map[string]string // pkgbase -> filename -> content, for the snapshot cache
}

// Update is an AUR package version change observed by Review. PackageBase is
// included because split packages share one reviewed source tree.
type Update struct {
	Name             string
	PackageBase      string
	InstalledVersion string
	TargetVersion    string
	LastModified     int64
}

// Review takes installed AUR packages (name -> installed version, from
// `pacman -Qm`) and returns findings ordered most-severe first. It does NOT
// write the baseline; call Commit after a trusted run.
func (r *Reviewer) Review(ctx context.Context, installed map[string]string) (*Result, error) {
	st, err := loadState(r.StatePath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(installed))
	for n := range installed {
		names = append(names, n)
	}
	sort.Strings(names)

	infos, err := r.fetchInfo(ctx, names)
	if err != nil {
		return nil, err
	}

	// Start the next baseline from what we already trusted, then layer the
	// freshly observed maintainers/versions on top.
	baseline := make(map[string]provenance, len(st.data))
	maps.Copy(baseline, st.data)

	var findings []Finding
	var updates []Update
	// Track maintainers that are NEW vs baseline this run -> cluster signal.
	newMaintainerHits := map[string][]string{}

	// A tracked package that vanished from the AUR was deleted or merged; it
	// will receive no further updates and deletions sometimes follow a malware
	// takedown. The baseline entry is kept so a re-submission under a different
	// account still trips the maintainer-change check.
	returned := make(map[string]bool, len(infos))
	for _, info := range infos {
		returned[info.Name] = true
	}
	for _, n := range names {
		if returned[n] {
			continue
		}
		if prev, known := st.data[n]; known {
			findings = append(findings, Finding{n, Warn, fmt.Sprintf(
				"no longer exists in the AUR (deleted or merged; last seen %s) — it will receive no updates; check why it was removed",
				time.Unix(prev.SeenAt, 0).Format("2006-01-02")), ""})
		}
	}

	var targets []rpcInfo
	targeted := map[string]bool{} // split packages share a pkgbase; fetch/scan it once

	for _, info := range infos {
		cur := ""
		if info.Maintainer != nil {
			cur = *info.Maintainer
		}
		prev, known := st.data[info.Name]
		pendingUpdate := installed[info.Name] != "" && installed[info.Name] != info.Version
		maintainerChanged := known && prev.Maintainer != "" && cur != prev.Maintainer
		// Orphan -> maintained is the classic takeover path (adopt, then push a
		// payload), so it gets its own HIGH rather than riding on
		// maintainerChanged, which deliberately ignores an empty previous value.
		adopted := known && prev.Maintainer == "" && cur != ""
		if pendingUpdate {
			updates = append(updates, Update{
				Name:             info.Name,
				PackageBase:      info.PackageBase,
				InstalledVersion: installed[info.Name],
				TargetVersion:    info.Version,
				LastModified:     info.LastModified,
			})
		}

		// (1) Provenance: maintainer change is the strongest takeover tell.
		if maintainerChanged {
			msg := fmt.Sprintf("maintainer changed: %q -> %q (last seen %s)",
				prev.Maintainer, maintOrOrphan(cur),
				time.Unix(prev.SeenAt, 0).Format("2006-01-02"))
			findings = append(findings, Finding{info.Name, High, msg, ""})
			if cur != "" {
				newMaintainerHits[cur] = append(newMaintainerHits[cur], info.Name)
			}
		}
		if adopted {
			findings = append(findings, Finding{info.Name, High, fmt.Sprintf(
				"adopted by %q after being orphaned (orphan last seen %s) — classic takeover path, review the diff closely",
				cur, time.Unix(prev.SeenAt, 0).Format("2006-01-02")), ""})
			newMaintainerHits[cur] = append(newMaintainerHits[cur], info.Name)
		}
		if cur == "" {
			findings = append(findings, Finding{info.Name, Warn,
				"package is currently ORPHANED — open to adoption/takeover", ""})
		}
		if info.OutOfDate != nil {
			findings = append(findings, Finding{info.Name, Info,
				fmt.Sprintf("flagged out-of-date since %s",
					time.Unix(*info.OutOfDate, 0).Format("2006-01-02")), ""})
		}

		// (2) Static scan, gated to packages that both need attention AND have
		// actually been pushed to since the last trusted run. Unchanged content
		// was already reviewed (VCS packages look perpetually "pending" but only
		// deserve a re-scan when someone pushes), and scanning every installed
		// package on every run is slow and unkind to the AUR.
		changedSinceTrust := !known || prev.LastModified != info.LastModified
		if (pendingUpdate || maintainerChanged || adopted || cur == "") && changedSinceTrust && !targeted[info.PackageBase] {
			targeted[info.PackageBase] = true
			targets = append(targets, info)
		}

		baseline[info.Name] = provenance{
			Maintainer: cur, Version: info.Version,
			SeenAt: time.Now().Unix(), LastModified: info.LastModified,
		}
	}

	scanFindings := make([][]Finding, len(targets))
	scanFiles := make([]map[string]string, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, scanWorkers)
	for i, info := range targets {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			scanFindings[i], scanFiles[i] = r.scanPackage(ctx, info)
		})
	}
	wg.Wait()

	files := map[string]map[string]string{}
	for i, info := range targets {
		findings = append(findings, scanFindings[i]...)
		if scanFiles[i] != nil {
			files[info.PackageBase] = scanFiles[i]
		}
	}

	// (3) Cluster detection: same new maintainer across multiple packages.
	for _, m := range slices.Sorted(maps.Keys(newMaintainerHits)) {
		pkgs := newMaintainerHits[m]
		if len(pkgs) >= 2 {
			sort.Strings(pkgs)
			findings = append(findings, Finding{
				Pkg:      strings.Join(pkgs, ","),
				Severity: High,
				Message: fmt.Sprintf("CLUSTER: %q just became maintainer of %d packages at once: %s",
					m, len(pkgs), strings.Join(pkgs, ", ")),
			})
		}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].Severity > findings[j].Severity
	})
	sort.Slice(updates, func(i, j int) bool { return updates[i].Name < updates[j].Name })
	return &Result{Findings: findings, Updates: updates, baseline: baseline, files: files}, nil
}

// Commit persists the baseline and file snapshots observed during Review. Run
// it only after the reviewed installed state is trusted so a rejected update
// stays flagged next time.
func (r *Reviewer) Commit(res *Result) error {
	if res == nil {
		return nil
	}
	if err := saveState(r.StatePath, res.baseline); err != nil {
		return err
	}
	return r.saveCaches(res.files)
}

func maintOrOrphan(m string) string {
	if m == "" {
		return "<orphan>"
	}
	return m
}

// ---- RPC ----

func (r *Reviewer) fetchInfo(ctx context.Context, names []string) ([]rpcInfo, error) {
	var out []rpcInfo
	for i := 0; i < len(names); i += rpcBatchLimit {
		end := min(i+rpcBatchLimit, len(names))
		q := url.Values{}
		for _, n := range names[i:end] {
			q.Add("arg[]", n)
		}
		body, err := r.get(ctx, rpcInfoURL+"?"+q.Encode(), maxFileBytes)
		if err != nil {
			return nil, err
		}
		var rr rpcResponse
		if err := json.Unmarshal(body, &rr); err != nil {
			return nil, fmt.Errorf("rpc decode: %w", err)
		}
		if rr.Type == "error" {
			return nil, fmt.Errorf("rpc error: %s", rr.Error)
		}
		out = append(out, rr.Results...)
	}
	return out, nil
}

// ---- Static scan ----

// scanPatterns map a regexp to a (severity, label). Tuned to surface, not to
// judge — expect false positives. fetch marks ecosystem downloads that get
// downgraded when the line pins to a lockfile (a pinned fetch is checksummed
// by the lockfile that ships in the checksummed sources).
var scanPatterns = []struct {
	re    *regexp.Regexp
	sev   Severity
	label string
	fetch bool
}{
	// Second-stage ecosystem fetches: the thing checksums DON'T cover.
	{regexp.MustCompile(`\b(npm|pnpm|yarn|bun)\s+(install|add|i|ci|x)\b`), High, "node-ecosystem fetch", true},
	{regexp.MustCompile(`\bpip[0-9]?\s+install\b`), High, "pip fetch", true},
	{regexp.MustCompile(`\bcargo\s+(install|add|fetch)\b`), High, "cargo fetch", true},
	{regexp.MustCompile(`\bgo\s+(install|get)\b`), High, "go fetch", true},
	{regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(ba|z|fi)?sh\b`), High, "pipe-to-shell", false},
	// Common payload-obfuscation and privilege tells.
	{regexp.MustCompile(`\bbase64\s+(-d|--decode)\b`), Warn, "base64 decode", false},
	{regexp.MustCompile(`\beval\b`), Warn, "eval", false},
	{regexp.MustCompile(`\bchmod\b[^\n]*\+s\b`), Warn, "setuid/setgid bit", false},
	// Install-time execution on YOUR host (outside the build sandbox).
	{regexp.MustCompile(`\b(pre|post)_(install|upgrade|remove)\s*\(\)`), Warn, "install/upgrade hook function", false},
	{regexp.MustCompile(`(?m)^\s*install\s*=`), Info, "declares an .install file", false},
	{regexp.MustCompile(`\.hook\b`), Warn, "references a .hook file", false},
}

// pinnedRE marks lockfile-pinned fetches (npm ci is lockfile-strict by design).
var pinnedRE = regexp.MustCompile(`--locked\b|--frozen-lockfile\b|--immutable\b|--require-hashes\b|--offline\b|\bnpm ci\b`)

var installDeclRE = regexp.MustCompile(`(?m)^\s*install\s*=\s*['"]?([^'"\s]+)`)

// skipSumRE matches the makepkg SKIP keyword. Case-sensitive on purpose: SKIP
// is always uppercase in a sums array, so this won't fire on prose like
// "Skip the foo step". Disabled verification is context-dependent (VCS sources
// legitimately use SKIP), so its findings are aggregated and Info, not High.
var skipSumRE = regexp.MustCompile(`\bSKIP\b`)

// urlHostRE pulls the host out of fetchable URLs so host drift (upstream
// hijack) can be flagged even when no scan pattern matches the line.
var urlHostRE = regexp.MustCompile(`(?i)\b(?:https?|git|ftp)://([^/'"\s)]+)`)

// scanPackage fetches the package's files and greps them for high-signal
// patterns. When a trusted snapshot exists in the cache, only lines that are
// new since that snapshot are flagged — everything else was already vetted on
// a previous run. Returns the findings plus the fetched files so Commit can
// persist them as the next trusted snapshot.
func (r *Reviewer) scanPackage(ctx context.Context, info rpcInfo) ([]Finding, map[string]string) {
	base := info.PackageBase
	files, binaries, err := r.fetchSnapshot(ctx, base)
	if err != nil {
		// Snapshot endpoint down or tarball unusable; fall back to fetching the
		// PKGBUILD (and declared .install) individually so coverage degrades
		// instead of disappearing.
		var looseErr error
		files, looseErr = r.fetchLoose(ctx, base)
		if looseErr != nil {
			return []Finding{{base, Warn,
				fmt.Sprintf("could not fetch files for scan: %v", errors.Join(err, looseErr)), ""}}, nil
		}
	}

	prev := r.loadCache(base)
	var findings []Finding
	for _, name := range binaries {
		findings = append(findings, Finding{base, Warn,
			"binary or oversized file in AUR repo (not scanned): " + name, ""})
	}

	for _, fname := range slices.Sorted(maps.Keys(files)) {
		prevLines := map[string]struct{}{}
		if prevContent, ok := prev[fname]; ok {
			for l := range strings.SplitSeq(prevContent, "\n") {
				prevLines[l] = struct{}{}
			}
		} else if len(prev) > 0 {
			findings = append(findings, Finding{base, Info,
				"new file since last trusted snapshot: " + fname, ""})
		}

		var skipLines []int
		for i, line := range strings.Split(files[fname], "\n") {
			// Comments are reminders, not behavior; skipping them kills the
			// bulk of the SKIP/pattern false positives (see spotify's PKGBUILD).
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			// Present verbatim in the trusted snapshot -> already vetted.
			if _, trusted := prevLines[line]; trusted {
				continue
			}
			for _, p := range scanPatterns {
				if !p.re.MatchString(line) {
					continue
				}
				sev, label := p.sev, p.label
				if p.fetch && pinnedRE.MatchString(line) {
					sev, label = Warn, label+" (lockfile-pinned)"
				}
				findings = append(findings, Finding{
					Pkg:      base,
					Severity: sev,
					Message:  label + ": " + strings.TrimSpace(line),
					Location: fmt.Sprintf("%s:%d", fname, i+1),
				})
			}
			if skipSumRE.MatchString(line) {
				skipLines = append(skipLines, i+1)
			}
		}
		// One aggregated SKIP finding per file instead of one per line.
		if len(skipLines) > 0 {
			findings = append(findings, Finding{
				Pkg:      base,
				Severity: Info,
				Message:  fmt.Sprintf("SKIP checksum on %d line(s) — verify no real source is left unchecked", len(skipLines)),
				Location: fname + ":" + joinInts(skipLines),
			})
		}
	}

	if prevPB, ok := prev["PKGBUILD"]; ok {
		findings = append(findings, hostDriftFindings(base, prevPB, files["PKGBUILD"])...)
	}
	return findings, files
}

// hostDriftFindings compares the URL hosts referenced by the trusted PKGBUILD
// against the incoming one. A swapped host is the upstream-hijack tell; a
// merely added host is worth a look but often benign (mirrors, extra sources).
func hostDriftFindings(pkg, prevPB, curPB string) []Finding {
	prev, cur := extractHosts(prevPB), extractHosts(curPB)
	var added, removed []string
	for h := range cur {
		if _, ok := prev[h]; !ok {
			added = append(added, h)
		}
	}
	for h := range prev {
		if _, ok := cur[h]; !ok {
			removed = append(removed, h)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	switch {
	case len(added) > 0 && len(removed) > 0:
		return []Finding{{pkg, High, fmt.Sprintf(
			"URL hosts changed: removed [%s], added [%s] — verify upstream wasn't hijacked",
			strings.Join(removed, ", "), strings.Join(added, ", ")), "PKGBUILD"}}
	case len(added) > 0:
		return []Finding{{pkg, Warn, "new URL host(s): " + strings.Join(added, ", "), "PKGBUILD"}}
	}
	return nil
}

func extractHosts(content string) map[string]struct{} {
	hosts := map[string]struct{}{}
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, m := range urlHostRE.FindAllStringSubmatch(line, -1) {
			h := strings.ToLower(m[1])
			if i := strings.LastIndexByte(h, '@'); i >= 0 {
				h = h[i+1:] // drop userinfo
			}
			if h != "" {
				hosts[h] = struct{}{}
			}
		}
	}
	return hosts
}

func joinInts(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

// ---- Fetching ----

// fetchSnapshot downloads the pkgbase's cgit snapshot tarball and returns its
// text files (path -> content) plus the names of binary/oversized files it
// refused to scan. Payloads regularly hide in patches and local scripts, so
// scanning only the PKGBUILD is not enough.
func (r *Reviewer) fetchSnapshot(ctx context.Context, pkgbase string) (map[string]string, []string, error) {
	u := fmt.Sprintf("%s/%s.tar.gz", cgitSnapshotURL, url.PathEscape(pkgbase))
	body, err := r.get(ctx, u, maxSnapshotBytes)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot decode: %w", err)
	}
	defer func() { _ = gz.Close() }()

	files := map[string]string{}
	var binaries []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("snapshot read: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// Entries are "<pkgbase>/<path>"; strip the top-level dir and refuse
		// anything that would escape it.
		slash := strings.IndexByte(hdr.Name, '/')
		if slash < 0 {
			continue
		}
		rel := hdr.Name[slash+1:]
		if rel == "" || !filepath.IsLocal(rel) {
			continue
		}
		if hdr.Size > maxFileBytes {
			binaries = append(binaries, rel)
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, fmt.Errorf("snapshot read %s: %w", rel, err)
		}
		if bytes.IndexByte(b, 0) >= 0 {
			binaries = append(binaries, rel)
			continue
		}
		files[rel] = string(b)
	}
	if _, ok := files["PKGBUILD"]; !ok {
		return nil, nil, fmt.Errorf("snapshot for %s has no PKGBUILD", pkgbase)
	}
	sort.Strings(binaries)
	return files, binaries, nil
}

// fetchLoose is the degraded path when the snapshot is unavailable: PKGBUILD
// plus the declared .install file, fetched individually from cgit plain.
func (r *Reviewer) fetchLoose(ctx context.Context, pkgbase string) (map[string]string, error) {
	pkgbuild, err := r.fetchPlain(ctx, pkgbase, "PKGBUILD")
	if err != nil {
		return nil, err
	}
	files := map[string]string{"PKGBUILD": pkgbuild}
	if m := installDeclRE.FindStringSubmatch(pkgbuild); m != nil {
		name := strings.NewReplacer("${pkgname}", pkgbase, "$pkgname", pkgbase).Replace(m[1])
		if body, err := r.fetchPlain(ctx, pkgbase, name); err == nil {
			files[name] = body
		}
	}
	return files, nil
}

func (r *Reviewer) fetchPlain(ctx context.Context, pkgbase, file string) (string, error) {
	u := fmt.Sprintf("%s/%s?h=%s", cgitPlainURL, url.PathEscape(file), url.QueryEscape(pkgbase))
	b, err := r.get(ctx, u, maxFileBytes)
	return string(b), err
}

// get issues a GET, enforces a 200 status, and caps the body at limit bytes.
func (r *Reviewer) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// ---- Snapshot cache ----

func (r *Reviewer) pkgCacheDir(pkgbase string) (string, bool) {
	if r.CacheDir == "" || !filepath.IsLocal(pkgbase) {
		return "", false
	}
	return filepath.Join(r.CacheDir, pkgbase), true
}

// loadCache returns the trusted snapshot from the last committed review. A
// missing or unreadable cache means "diff against nothing", i.e. a full scan.
func (r *Reviewer) loadCache(pkgbase string) map[string]string {
	dir, ok := r.pkgCacheDir(pkgbase)
	if !ok {
		return nil
	}
	out := map[string]string{}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() > maxFileBytes {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return out
}

// saveCaches replaces each scanned package's trusted snapshot with the files
// observed this run. Only called from Commit, i.e. after a trusted run.
func (r *Reviewer) saveCaches(files map[string]map[string]string) error {
	for _, base := range slices.Sorted(maps.Keys(files)) {
		dir, ok := r.pkgCacheDir(base)
		if !ok {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		for name, content := range files[base] {
			if !filepath.IsLocal(name) {
				continue
			}
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}
