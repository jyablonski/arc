package sysupdate

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

type aurOutputMode uint8

const (
	aurOutputCompact aurOutputMode = iota
	aurOutputReview
)

var (
	aurSelectionLine       = regexp.MustCompile(`^\s*\d+\s+\S+\s+.*(?:->|\([^)]*\))`)
	aurReviewSelectionLine = regexp.MustCompile(`^\s*\d+\s+\S+(?:\s|$)`)
)

// aurOutput keeps yay's complete output in the run log while reducing its
// terminal stream to plans, the automatically selected build-file diffs,
// interactive gates, promoted warnings, and transient phase updates. Diffs
// are rendered inline so they remain visible before the install prompt.
type aurOutput struct {
	mu sync.Mutex

	log      io.Writer
	renderer Renderer
	out      io.Writer

	mode          aurOutputMode
	pending       string
	reviewPending string
	selections    []string
	prompt        string
	dependency    bool
	installPlan   bool
	packageName   string
	diffPackages  []string
	diffPackage   string
	diffFile      string
	warnings      map[string]struct{}
	stopProgress  func()
}

func newAUROutput(log io.Writer, renderer Renderer, diffPackages ...string) *aurOutput {
	return &aurOutput{
		log:          log,
		renderer:     renderer,
		out:          renderer.writer(),
		diffPackages: append([]string(nil), diffPackages...),
		warnings:     make(map[string]struct{}),
		stopProgress: func() {},
	}
}

func (w *aurOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.log.Write(p); err != nil {
		return 0, err
	}
	w.consume(p)
	return len(p), nil
}

func (w *aurOutput) Finish() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.reviewPending != "" {
		w.processReviewLine(w.reviewPending)
		w.reviewPending = ""
	}
	if line := strings.TrimSpace(sanitizeTerminal(w.pending)); line != "" {
		w.processLine(line)
	}
	w.pending = ""
	w.stopProgress()
	w.stopProgress = func() {}
}

func (w *aurOutput) consume(p []byte) {
	for len(p) > 0 {
		if w.mode != aurOutputCompact {
			p = w.consumeReview(p)
			continue
		}

		i := strings.IndexAny(string(p), "\r\n")
		if i < 0 {
			w.pending += string(p)
			w.showPartialPrompt()
			return
		}
		w.pending += string(p[:i])
		w.processLine(w.pending)
		w.pending = ""
		p = p[i+1:]
	}
}

func (w *aurOutput) consumeReview(p []byte) []byte {
	combined := []byte(w.reviewPending + string(p))
	w.reviewPending = ""
	markers := []string{
		"==> PKGBUILDs to edit?", "==> Making package:",
		":: Synchronizing package databases", ":: Parsing SRCINFO:", "Parsing SRCINFO:",
		":: Proceed with install?", ":: Proceed with installation?",
	}
	if index := firstMarker(combined, markers); index >= 0 {
		w.processReviewBytes(combined[:index])
		if w.reviewPending != "" {
			w.processReviewLine(w.reviewPending)
			w.reviewPending = ""
		}
		w.mode = aurOutputCompact
		w.diffPackage = ""
		w.diffFile = ""
		return combined[index:]
	}

	w.processReviewBytes(combined)
	return nil
}

func (w *aurOutput) processReviewBytes(p []byte) {
	for len(p) > 0 {
		i := strings.IndexAny(string(p), "\r\n")
		if i < 0 {
			w.reviewPending += string(p)
			return
		}
		w.reviewPending += string(p[:i])
		w.processReviewLine(w.reviewPending)
		w.reviewPending = ""
		p = p[i+1:]
	}
}

func firstMarker(p []byte, markers []string) int {
	first := -1
	for _, marker := range markers {
		if index := strings.Index(string(p), marker); index >= 0 && (first < 0 || index < first) {
			first = index
		}
	}
	return first
}

func (w *aurOutput) processReviewLine(raw string) {
	line := sanitizeTerminal(raw)
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "diff --git ") {
		pkg := w.diffPackageFromHeader(trimmed)
		if pkg != "" && pkg != w.diffPackage {
			w.diffPackage = pkg
			w.diffFile = ""
			w.renderer.DiffPackage(pkg)
		}
		file := diffFileFromHeader(trimmed, pkg)
		if file != "" && file != w.diffFile {
			w.diffFile = file
			w.renderer.DiffFile(file)
		}
		return
	}
	if w.diffPackage == "" || trimmed == "" || aurReviewSelectionLine.MatchString(trimmed) || strings.HasPrefix(trimmed, "==>") {
		return
	}
	if strings.HasPrefix(trimmed, "index ") || strings.HasPrefix(trimmed, "--- ") || strings.HasPrefix(trimmed, "+++ ") {
		return
	}
	w.renderer.DiffLine(line)
}

func (w *aurOutput) diffPackageFromHeader(header string) string {
	for _, pkg := range w.diffPackages {
		if strings.Contains(header, "/"+pkg+"/") {
			return pkg
		}
	}
	if len(w.diffPackages) == 1 {
		return w.diffPackages[0]
	}
	return "AUR package"
}

func diffFileFromHeader(header, pkg string) string {
	if pkg != "" && pkg != "AUR package" {
		marker := "/" + pkg + "/"
		if i := strings.Index(header, marker); i >= 0 {
			rest := header[i+len(marker):]
			if fields := strings.Fields(rest); len(fields) > 0 {
				return strings.Trim(fields[0], `"`)
			}
		}
	}
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return ""
	}
	return strings.TrimPrefix(strings.Trim(fields[2], `"`), "a/")
}

func (w *aurOutput) processLine(raw string) {
	line := strings.TrimSpace(sanitizeTerminal(raw))
	if line == "" {
		w.dependency = false
		return
	}

	if strings.HasPrefix(line, "==> WARNING:") {
		message := strings.TrimSpace(strings.TrimPrefix(line, "==> WARNING:"))
		if isRoutineAURWarning(message) {
			return
		}
		if w.packageName != "" {
			message = w.packageName + ": " + message
		}
		w.promoteWarning(message)
		return
	}
	if strings.HasPrefix(line, "==> ERROR:") || strings.HasPrefix(line, "error:") {
		w.stopPhase()
		w.renderer.Error(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "==> ERROR:"), "error:")))
		return
	}
	if w.prompt != "" && strings.HasPrefix(line, "==>") && !strings.HasPrefix(line, "==> [") {
		w.completeMenuPrompt()
		return
	}
	if isInlineAURPrompt(line) {
		w.stopPhase()
		w.installPlan = false
		w.flushSelections()
		w.nativeLine(line)
		return
	}

	if aurSelectionLine.MatchString(line) {
		w.selections = append(w.selections, line)
		if len(w.selections) > 20 {
			w.selections = w.selections[len(w.selections)-20:]
		}
		return
	}

	if isAURMenuHeading(line) {
		w.stopPhase()
		w.selections = nil
		w.prompt = line
		return
	}
	if w.prompt != "" && strings.HasPrefix(line, "==> [") {
		return
	}

	if strings.HasPrefix(line, ":: ") && strings.Contains(line, "dependencies will also be installed") {
		w.stopPhase()
		w.renderer.Info(strings.TrimPrefix(line, ":: "))
		w.dependency = true
		return
	}
	if w.dependency && (strings.HasPrefix(line, "extra/") || strings.HasPrefix(line, "aur/") || strings.HasPrefix(line, "multilib/") || strings.HasPrefix(line, "core/")) {
		_, _ = fmt.Fprintf(w.out, "    %s\n", line)
		return
	}
	if packageName := aurPackageName(line); packageName != "" {
		w.packageName = packageName
	}
	if phase := aurPhase(line); phase != "" {
		w.installPlan = false
		w.setPhase(phase)
		return
	}

	if strings.HasPrefix(line, "Packages (") {
		w.stopPhase()
		w.installPlan = true
		w.nativeLine(line)
		return
	}
	if w.installPlan && (strings.HasPrefix(line, "Total ") || strings.HasPrefix(line, "Net ")) {
		w.nativeLine(line)
		return
	}
	if w.installPlan {
		w.nativeLine(line)
		return
	}
}

func isRoutineAURWarning(message string) bool {
	return strings.HasPrefix(message, "Using existing $srcdir/ tree")
}

func aurPackageName(line string) string {
	const marker = "Making package:"
	index := strings.Index(line, marker)
	if index < 0 {
		return ""
	}
	fields := strings.Fields(line[index+len(marker):])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (w *aurOutput) showPartialPrompt() {
	line := strings.TrimSpace(sanitizeTerminal(w.pending))
	if line == "" {
		return
	}
	if w.prompt != "" && strings.HasSuffix(line, "==>") {
		w.completeMenuPrompt()
		w.pending = ""
		return
	}
	if isInlineAURPrompt(line) {
		w.stopPhase()
		w.installPlan = false
		w.flushSelections()
		_, _ = fmt.Fprintf(w.out, "  %s ", line)
		w.pending = ""
	}
}

func (w *aurOutput) completeMenuPrompt() {
	prompt := w.prompt
	w.prompt = ""
	if strings.Contains(prompt, "Diffs to show?") {
		w.mode = aurOutputReview
	}
}

func isAURMenuHeading(line string) bool {
	for _, heading := range []string{"Packages to exclude:", "Packages to cleanBuild?", "Diffs to show?", "PKGBUILDs to edit?"} {
		if strings.Contains(line, heading) {
			return true
		}
	}
	return false
}

func isInlineAURPrompt(line string) bool {
	if strings.Contains(line, "[Y/n]") || strings.Contains(line, "[y/N]") {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasSuffix(line, ":") && (strings.Contains(lower, "enter a selection") || strings.Contains(lower, "select a provider"))
}

func aurPhase(line string) string {
	switch {
	case strings.Contains(line, "Downloaded PKGBUILD"):
		return "fetching AUR build files…"
	case strings.Contains(line, "Retrieving sources"):
		return "fetching AUR sources…"
	case strings.Contains(line, "Validating source files"):
		return "verifying AUR sources…"
	case strings.Contains(line, "Starting prepare()"):
		return "preparing AUR sources…"
	case strings.Contains(line, "Starting check()"):
		return "running AUR package checks…"
	case strings.Contains(line, "Starting package()") || strings.Contains(line, "Creating package "):
		return "packaging AUR updates…"
	case strings.Contains(line, "Starting build()") || strings.Contains(line, "Making package:"):
		return "building AUR updates…"
	case strings.Contains(line, "Retrieving packages"):
		return "downloading dependencies…"
	case strings.Contains(line, "Synchronizing package databases") || strings.Contains(line, "resolving dependencies"):
		return "resolving dependencies…"
	case strings.Contains(line, "Processing package changes"):
		return "installing packages…"
	default:
		return ""
	}
}

func (w *aurOutput) promoteWarning(message string) {
	if _, seen := w.warnings[message]; seen {
		return
	}
	w.warnings[message] = struct{}{}
	w.stopPhase()
	w.renderer.Warning(message)
}

func (w *aurOutput) setPhase(message string) {
	w.stopProgress()
	w.stopProgress = w.renderer.Progress(message)
}

func (w *aurOutput) stopPhase() {
	w.stopProgress()
	w.stopProgress = func() {}
}

func (w *aurOutput) flushSelections() {
	for _, line := range w.selections {
		w.nativeLine(line)
	}
	w.selections = nil
}

func (w *aurOutput) nativeLine(line string) {
	_, _ = fmt.Fprintf(w.out, "  %s\n", strings.TrimSpace(line))
}
