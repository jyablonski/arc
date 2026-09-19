package aurreview

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Change summarizes how a package's AUR files moved since the last trusted
// snapshot, so a routine version bump reads as one line instead of a diff.
type Change struct {
	// Summary names what changed, e.g. "pkgver + checksums" or
	// "source + package()".
	Summary string
	// Routine is true when nothing that decides what gets built or installed
	// changed: only version, checksum, and descriptive metadata fields.
	Routine bool
	// Baseline is false when there was no trusted snapshot to diff against.
	Baseline bool
	// Files are unified diffs of every changed file, for review on demand.
	Files []FileDiff
}

// FileDiff is one file's unified diff lines ("@@", "+", "-", " " prefixed).
type FileDiff struct {
	Name  string
	Lines []string
}

// Change categories, in display order. The routine ones never decide what
// gets fetched, built, or run on the host.
const (
	catVersion   = "pkgver"
	catChecksums = "checksums"
	catMetadata  = "metadata"
	catDepends   = "depends"
	catSource    = "source"
	catInstall   = "install"
	catOther     = "other"
)

var routineCategories = map[string]bool{catVersion: true, catChecksums: true, catMetadata: true}

var (
	assignRE   = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)(\[[^]]*\])?\+?=`)
	functionRE = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_-]*)\s*\(\)\s*\{?\s*$`)
	// versionishVarRE matches maintainer helper variables that track the
	// upstream version (_commit, _pkgver, _build, …).
	versionishVarRE = regexp.MustCompile(`(?i)(ver|commit|rev|tag|build|sha|hash|date|release)`)
	checksumVarRE   = regexp.MustCompile(`^(md5|sha1|sha224|sha256|sha384|sha512|b2|ck)sums(_.+)?$`)
)

var depVars = map[string]bool{
	"depends": true, "makedepends": true, "checkdepends": true, "optdepends": true,
}

var metadataVars = map[string]bool{
	"pkgdesc": true, "url": true, "license": true, "arch": true, "groups": true,
	"provides": true, "conflicts": true, "replaces": true, "options": true,
	"backup": true, "changelog": true, "pkgname": true, "pkgbase": true,
}

// metadataFiles never influence a build.
var metadataFiles = map[string]bool{
	".gitignore": true, ".nvchecker.toml": true, "REUSE.toml": true, "LICENSE": true,
	"LICENSE.txt": true, "README.md": true, ".editorconfig": true,
}

// classify compares the trusted snapshot (prev) with the incoming files (cur).
func classify(prev, cur map[string]string) Change {
	c := Change{Baseline: len(prev) > 0}
	if !c.Baseline {
		for _, name := range slices.Sorted(maps.Keys(cur)) {
			if name == ".SRCINFO" {
				continue
			}
			c.Files = append(c.Files, FileDiff{Name: name, Lines: unifiedDiff("", cur[name])})
		}
		c.Summary = "no trusted snapshot"
		return c
	}

	cats := map[string]bool{}
	var extra []string // non-PKGBUILD files and function names, shown verbatim
	depsAdded := false
	names := map[string]struct{}{}
	for n := range prev {
		names[n] = struct{}{}
	}
	for n := range cur {
		names[n] = struct{}{}
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		if name == ".SRCINFO" { // generated from PKGBUILD
			continue
		}
		before, after := prev[name], cur[name]
		if before == after {
			continue
		}
		c.Files = append(c.Files, FileDiff{Name: name, Lines: unifiedDiff(before, after)})
		switch {
		case name == "PKGBUILD":
			fileCats, funcs, added := classifyPKGBUILD(before, after)
			for k := range fileCats {
				cats[k] = true
			}
			extra = append(extra, funcs...)
			depsAdded = depsAdded || added
		case metadataFiles[name]:
			cats[catMetadata] = true
		default:
			extra = append(extra, fileLabel(name, prev, cur))
		}
	}

	if len(c.Files) == 0 {
		c.Summary = "unchanged since last review"
		c.Routine = true
		return c
	}

	var parts []string
	for _, k := range []string{catVersion, catChecksums, catMetadata, catDepends, catSource, catInstall, catOther} {
		if cats[k] {
			parts = append(parts, k)
		}
	}
	parts = append(parts, extra...)
	c.Summary = strings.Join(parts, " + ")

	c.Routine = len(extra) == 0 && !depsAdded
	for k := range cats {
		if !routineCategories[k] && k != catDepends {
			c.Routine = false
		}
	}
	return c
}

func fileLabel(name string, prev, cur map[string]string) string {
	_, had := prev[name]
	_, has := cur[name]
	switch {
	case !had:
		return "+" + name
	case !has:
		return "-" + name
	default:
		return name
	}
}

// classifyPKGBUILD attributes every changed line to the variable or function
// it belongs to. Functions are reported by name ("package()") because any
// change there is build logic; depsAdded is set only when a dependency line
// was added, since dropping one cannot pull new code in.
func classifyPKGBUILD(before, after string) (cats map[string]bool, funcs []string, depsAdded bool) {
	cats = map[string]bool{}
	seenFunc := map[string]bool{}
	oldOwners, newOwners := lineOwners(before), lineOwners(after)
	for _, op := range diffOps(splitLines(before), splitLines(after)) {
		if op.kind == ' ' {
			continue
		}
		line := op.line
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		var owner string
		if op.kind == '+' {
			owner = newOwners[op.newIdx]
		} else {
			owner = oldOwners[op.oldIdx]
		}
		switch {
		case strings.HasSuffix(owner, "()"):
			if !seenFunc[owner] {
				seenFunc[owner] = true
				funcs = append(funcs, owner)
			}
		case owner == "pkgver" || owner == "pkgrel" || owner == "epoch" ||
			(strings.HasPrefix(owner, "_") && versionishVarRE.MatchString(owner)):
			cats[catVersion] = true
		case checksumVarRE.MatchString(owner):
			cats[catChecksums] = true
		case depVars[owner]:
			cats[catDepends] = true
			if op.kind == '+' {
				depsAdded = true
			}
		case owner == "source" || strings.HasPrefix(owner, "source_") || owner == "validpgpkeys" || owner == "noextract":
			cats[catSource] = true
		case owner == "install":
			cats[catInstall] = true
		case metadataVars[owner]:
			cats[catMetadata] = true
		default:
			cats[catOther] = true
		}
	}
	return cats, funcs, depsAdded
}

// lineOwners maps each line index to the top-level construct containing it:
// a variable name for assignments (including multi-line arrays), "name()" for
// function bodies, or "" for anything else.
func lineOwners(content string) []string {
	lines := splitLines(content)
	owners := make([]string, len(lines))
	// awaitBrace covers the "package()\n{" style where the body opens on the
	// line after the header.
	current, depth, inArray, awaitBrace := "", 0, false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case awaitBrace && strings.HasPrefix(trimmed, "{"):
			awaitBrace = false
		case depth > 0:
		case inArray:
			if strings.Contains(trimmed, ")") && balanced(trimmed) {
				inArray = false
			}
			owners[i] = current
			continue
		default:
			awaitBrace = false
			current = ""
			if m := functionRE.FindStringSubmatch(line); m != nil {
				current = m[1] + "()"
				awaitBrace = !strings.Contains(line, "{")
			} else if m := assignRE.FindStringSubmatch(line); m != nil {
				current = m[1]
				rest := strings.TrimSpace(line[len(m[0]):])
				inArray = strings.HasPrefix(rest, "(") && !balanced(rest)
			}
		}
		owners[i] = current
		if strings.HasSuffix(current, "()") {
			depth = max(0, depth+strings.Count(line, "{")-strings.Count(line, "}"))
		}
	}
	return owners
}

func balanced(s string) bool {
	return strings.Count(s, "(") <= strings.Count(s, ")")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

type diffOp struct {
	kind           byte // ' ', '-', '+'
	line           string
	oldIdx, newIdx int
}

// maxDiffCells bounds the LCS table; PKGBUILDs are small, but a vendored
// patch could be large enough to make a quadratic table expensive.
const maxDiffCells = 4_000_000

// diffOps is a line-level LCS diff. Past maxDiffCells it degrades to "all
// old lines removed, all new lines added", which is still a correct diff.
func diffOps(a, b []string) []diffOp {
	if len(a)*len(b) > maxDiffCells {
		ops := make([]diffOp, 0, len(a)+len(b))
		for i, l := range a {
			ops = append(ops, diffOp{kind: '-', line: l, oldIdx: i})
		}
		for j, l := range b {
			ops = append(ops, diffOp{kind: '+', line: l, newIdx: j})
		}
		return ops
	}
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{kind: ' ', line: a[i], oldIdx: i, newIdx: j})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{kind: '-', line: a[i], oldIdx: i, newIdx: j})
			i++
		default:
			ops = append(ops, diffOp{kind: '+', line: b[j], oldIdx: i, newIdx: j})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{kind: '-', line: a[i], oldIdx: i, newIdx: j})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{kind: '+', line: b[j], oldIdx: i, newIdx: j})
	}
	return ops
}

// unifiedDiff renders a diff with three lines of context per hunk.
func unifiedDiff(before, after string) []string {
	const context = 3
	ops := diffOps(splitLines(before), splitLines(after))
	var out []string
	for start := 0; start < len(ops); {
		// Find the next change.
		first := -1
		for k := start; k < len(ops); k++ {
			if ops[k].kind != ' ' {
				first = k
				break
			}
		}
		if first < 0 {
			break
		}
		// Extend the hunk while changes are within 2*context of each other.
		lo, hi := max(0, first-context), first
		for k := first; k < len(ops); k++ {
			if ops[k].kind != ' ' {
				hi = k
			} else if k-hi > 2*context {
				break
			}
		}
		hi = min(len(ops)-1, hi+context)

		oldStart, newStart, oldN, newN := ops[lo].oldIdx+1, ops[lo].newIdx+1, 0, 0
		var body []string
		for _, op := range ops[lo : hi+1] {
			body = append(body, string(op.kind)+op.line)
			if op.kind != '+' {
				oldN++
			}
			if op.kind != '-' {
				newN++
			}
		}
		out = append(out, fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldN, newStart, newN))
		out = append(out, body...)
		start = hi + 1
	}
	return out
}
