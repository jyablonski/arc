// Package aurreview adds a triage layer on top of yay's interactive diffmenu:
//  1. provenance baseline tracking (maintainer-change = takeover signal)
//  2. static scan of the incoming PKGBUILD/.install for high-signal patterns
//  3. cross-package cluster detection (one account touching many at once)
//
// It NEVER decides for you. It surfaces findings and routes your attention;
// you still make the call at the diffmenu. grep-based scanning has false
// positives (e.g. a SKIP sum that's overridden on the next line) and is
// defeated by obfuscation, so treat HIGH findings as "look here first",
// not "reject".
//
// Review computes findings without touching the baseline; Commit persists the
// observed maintainers/versions and is meant to run only after yay actually
// applied the updates. That ordering matters: committing before you review
// would let a takeover silently rewrite the "known good" maintainer, so it
// would never flag again.
package aurreview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/boundary"
)

const (
	rpcInfoURL    = "https://aur.archlinux.org/rpc/v5/info"
	cgitPlainURL  = "https://aur.archlinux.org/cgit/aur.git/plain"
	rpcBatchLimit = 150 // official instance caps very large info requests
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
	Maintainer string `json:"maintainer"`
	Version    string `json:"version"`
	SeenAt     int64  `json:"seen_at"`
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
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "arc", "aur-provenance.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "arc", "aur-provenance.json"), nil
}

// ---- Reviewer ----

// Reviewer fetches AUR metadata and produces triage findings.
type Reviewer struct {
	HTTP      boundary.HTTPDoer
	StatePath string
}

// New returns a Reviewer with a 15s HTTP client writing its baseline to statePath.
func New(statePath string) *Reviewer {
	return &Reviewer{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		StatePath: statePath,
	}
}

// Result carries the findings plus the baseline that Commit should persist if
// the run is trusted (yay actually applied the updates). Pending is the count
// of packages whose installed version differs from the AUR's (i.e. real
// upgrades), so callers can stay quiet when there's nothing to upgrade.
type Result struct {
	Findings []Finding
	Pending  int
	baseline map[string]provenance
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
	pending := 0
	// Track maintainers that are NEW vs baseline this run -> cluster signal.
	newMaintainerHits := map[string][]string{}

	for _, info := range infos {
		cur := ""
		if info.Maintainer != nil {
			cur = *info.Maintainer
		}
		prev, known := st.data[info.Name]
		pendingUpdate := installed[info.Name] != "" && installed[info.Name] != info.Version
		maintainerChanged := known && prev.Maintainer != "" && cur != prev.Maintainer
		if pendingUpdate {
			pending++
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
		if cur == "" {
			findings = append(findings, Finding{info.Name, Warn,
				"package is currently ORPHANED — open to adoption/takeover", ""})
		}
		if info.OutOfDate != nil {
			findings = append(findings, Finding{info.Name, Info,
				fmt.Sprintf("flagged out-of-date since %s",
					time.Unix(*info.OutOfDate, 0).Format("2006-01-02")), ""})
		}

		// (2) Static scan, gated to packages actually changing or suspicious.
		// Scanning every installed PKGBUILD on every run is slow and unkind to
		// the AUR; the payload you care about ships with a version bump or a
		// maintainer flip.
		if pendingUpdate || maintainerChanged || cur == "" {
			findings = append(findings, r.scanPackage(ctx, info)...)
		}

		baseline[info.Name] = provenance{Maintainer: cur, Version: info.Version, SeenAt: time.Now().Unix()}
	}

	// (3) Cluster detection: same new maintainer across multiple packages.
	for m, pkgs := range newMaintainerHits {
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
	return &Result{Findings: findings, Pending: pending, baseline: baseline}, nil
}

// Commit persists the baseline observed during Review. Run it only after a
// trusted run so a rejected takeover stays flagged next time.
func (r *Reviewer) Commit(res *Result) error {
	if res == nil {
		return nil
	}
	return saveState(r.StatePath, res.baseline)
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
		body, err := r.get(ctx, rpcInfoURL+"?"+q.Encode())
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
// judge — expect false positives.
var scanPatterns = []struct {
	re    *regexp.Regexp
	sev   Severity
	label string
}{
	// Second-stage ecosystem fetches: the thing checksums DON'T cover.
	{regexp.MustCompile(`\b(npm|pnpm|yarn|bun)\s+(install|add|i|x)\b`), High, "node-ecosystem fetch"},
	{regexp.MustCompile(`\bpip[0-9]?\s+install\b`), High, "pip fetch"},
	{regexp.MustCompile(`\bcargo\s+(install|add)\b`), High, "cargo fetch"},
	{regexp.MustCompile(`\bgo\s+(install|get)\b`), High, "go fetch"},
	{regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(ba|z|fi)?sh\b`), High, "pipe-to-shell"},
	// Install-time execution on YOUR host (outside the build sandbox).
	{regexp.MustCompile(`\b(pre|post)_(install|upgrade|remove)\s*\(\)`), Warn, "install/upgrade hook function"},
	{regexp.MustCompile(`(?m)^\s*install\s*=`), Info, "declares an .install file"},
	{regexp.MustCompile(`\.hook\b`), Warn, "references a .hook file"},
}

var installDeclRE = regexp.MustCompile(`(?m)^\s*install\s*=\s*['"]?([^'"\s]+)`)

// skipSumRE matches the makepkg SKIP keyword. Case-sensitive on purpose: SKIP
// is always uppercase in a sums array, so this won't fire on prose like
// "Skip the foo step". Disabled verification is context-dependent (VCS sources
// legitimately use SKIP), so its findings are aggregated and Info, not High.
var skipSumRE = regexp.MustCompile(`\bSKIP\b`)

func (r *Reviewer) scanPackage(ctx context.Context, info rpcInfo) []Finding {
	var findings []Finding
	files := []string{"PKGBUILD"}

	pkgbuild, err := r.fetchPlain(ctx, info.PackageBase, "PKGBUILD")
	if err != nil {
		return []Finding{{info.Name, Warn,
			fmt.Sprintf("could not fetch PKGBUILD for scan: %v", err), ""}}
	}
	contents := map[string]string{"PKGBUILD": pkgbuild}

	// If the PKGBUILD declares install=NAME, also pull that file.
	if m := installDeclRE.FindStringSubmatch(pkgbuild); m != nil {
		name := strings.NewReplacer("${pkgname}", info.PackageBase, "$pkgname", info.PackageBase).Replace(m[1])
		if body, err := r.fetchPlain(ctx, info.PackageBase, name); err == nil {
			files = append(files, name)
			contents[name] = body
		}
	}

	for _, fname := range files {
		var skipLines []int
		for i, line := range strings.Split(contents[fname], "\n") {
			// Comments are reminders, not behavior; skipping them kills the
			// bulk of the SKIP/pattern false positives (see spotify's PKGBUILD).
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, p := range scanPatterns {
				if p.re.MatchString(line) {
					findings = append(findings, Finding{
						Pkg:      info.Name,
						Severity: p.sev,
						Message:  p.label + ": " + strings.TrimSpace(line),
						Location: fmt.Sprintf("%s:%d", fname, i+1),
					})
				}
			}
			if skipSumRE.MatchString(line) {
				skipLines = append(skipLines, i+1)
			}
		}
		// One aggregated SKIP finding per file instead of one per line.
		if len(skipLines) > 0 {
			findings = append(findings, Finding{
				Pkg:      info.Name,
				Severity: Info,
				Message:  fmt.Sprintf("SKIP checksum on %d line(s) — verify no real source is left unchecked", len(skipLines)),
				Location: fname + ":" + joinInts(skipLines),
			})
		}
	}
	return findings
}

func joinInts(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func (r *Reviewer) fetchPlain(ctx context.Context, pkgbase, file string) (string, error) {
	u := fmt.Sprintf("%s/%s?h=%s", cgitPlainURL, url.PathEscape(file), url.QueryEscape(pkgbase))
	b, err := r.get(ctx, u)
	return string(b), err
}

// get issues a GET, enforces a 200 status, and caps the body at 1MiB.
func (r *Reviewer) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
