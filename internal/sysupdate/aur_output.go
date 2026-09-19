package sysupdate

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jyablonski/arc/internal/output"
)

type aurOutputMode uint8

const (
	aurOutputCompact aurOutputMode = iota
	aurOutputReview
)

var (
	aurSelectionLine       = regexp.MustCompile(`^\s*\d+\s+(\S+)\s+.*(?:->|\([^)]*\))`)
	aurReviewSelectionLine = regexp.MustCompile(`^\s*\d+\s+\S+(?:\s|$)`)
	// yay counts its operations inline, e.g. ":: (1/1) Parsing SRCINFO: foo".
	aurOperationCounter = regexp.MustCompile(`^:: \(\d+/\d+\) `)
	// yay's dependency summary, e.g. "AUR Dependency (2): foo-1.0-1, bar-2-1".
	aurDependencyList = regexp.MustCompile(`^AUR (?:Dependency|Explicit|Make Dependency|Check Dependency) \(\d+\):\s*(.*)$`)
	netUpgradeSize    = regexp.MustCompile(`^Net Upgrade Size:\s*(-?[\d.]+)\s*(B|KiB|MiB|GiB|TiB)$`)
	// promptTail marks unterminated output that is probably waiting for input:
	// a question, a bracketed choice, or an instruction to answer. It is
	// deliberately narrower than "ends in a colon" so a build that pauses
	// after printing "Compiling foo:" is not mistaken for a prompt.
	promptTail = regexp.MustCompile(`(?i)(\?\s*$|\[[yn]/[yn]\]\s*$|\b(enter|select|choose|provide|password|option|number|answer|continue|proceed)\b[^\n]*[:?]\s*$)`)
)

// maxInstallPlanLines bounds what arc holds from pacman's transaction plan to
// replay at a surfaced gate, in case the plan is never followed by one.
const maxInstallPlanLines = 40

// unknownPromptDelay is how long unterminated, question-shaped output may sit
// before arc treats it as a prompt it does not recognise and hands it to the
// person, so an unexpected question never deadlocks the proxied stdin.
const unknownPromptDelay = 3 * time.Second

// aurOutputOptions configures the yay proxy.
type aurOutputOptions struct {
	// diffPackages are pkgbases whose yay-rendered diffs are expected; only
	// used on the fallback path where arc's own review was unavailable.
	diffPackages []string
	// answers is yay's stdin. arc writes every answer yay and pacman receive.
	answers io.Writer
	// input is the person's terminal input for prompts arc surfaces.
	input *bufio.Reader
	// approved are the package names approved at arc's prompt. nil means no
	// arc-level approval happened and every gate is surfaced.
	approved map[string]bool
	now      func() time.Time
}

// aurOutput keeps yay's complete output in the run log while reducing its
// terminal stream to arc-worded prompts, promoted warnings and errors, and
// transient phase updates. It also owns yay's stdin: the install gates yay and
// pacman raise are answered automatically only when the transaction matches
// the plan the person approved; anything else is surfaced as a real prompt.
type aurOutput struct {
	mu sync.Mutex

	log      io.Writer
	renderer Renderer
	out      io.Writer
	opts     aurOutputOptions

	mode          aurOutputMode
	pending       string
	reviewPending string
	selections    []string
	prompt        string
	dependency    bool
	installPlan   bool
	installLines  []string
	installNames  []string
	unapproved    map[string]bool
	packageName   string
	diffPackage   string
	diffFile      string
	warnings      map[string]struct{}
	stopProgress  func()

	netBytes    int64
	builds      map[string]time.Duration
	buildName   string
	buildStart  time.Time
	promptTimer *time.Timer
	promptGen   int
}

func newAUROutput(log io.Writer, renderer Renderer, opts aurOutputOptions) *aurOutput {
	if opts.now == nil {
		opts.now = time.Now
	}
	if opts.answers == nil {
		opts.answers = io.Discard
	}
	if opts.input == nil {
		opts.input = bufio.NewReader(strings.NewReader(""))
	}
	return &aurOutput{
		log:          log,
		renderer:     renderer,
		out:          renderer.writer(),
		opts:         opts,
		unapproved:   make(map[string]bool),
		warnings:     make(map[string]struct{}),
		builds:       make(map[string]time.Duration),
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

	w.cancelPromptTimer()
	if w.reviewPending != "" {
		w.processReviewLine(w.reviewPending)
		w.reviewPending = ""
	}
	if line := strings.TrimSpace(sanitizeTerminal(w.pending)); line != "" {
		w.processLine(line)
	}
	w.pending = ""
	w.closeBuild()
	w.stopProgress()
	w.stopProgress = func() {}
}

// BuildTimes reports how long each pkgbase took from "Making package" to the
// next build or the install transaction.
func (w *aurOutput) BuildTimes() map[string]time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.builds
}

// NetBytes is pacman's reported net upgrade size across AUR transactions.
func (w *aurOutput) NetBytes() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.netBytes
}

func (w *aurOutput) consume(p []byte) {
	w.cancelPromptTimer()
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
	for len(p) > 0 {
		i := strings.IndexAny(string(p), "\r\n")
		if i < 0 {
			i = len(p)
		}
		w.reviewPending += string(p[:i])
		p = p[i:]

		// Match sanitized text, but retain raw bytes so split escape sequences
		// and unterminated prompts survive the handoff to compact mode.
		if isAURReviewEnd(sanitizeTerminal(w.reviewPending)) {
			w.mode = aurOutputCompact
			w.diffPackage = ""
			w.diffFile = ""
			p = append([]byte(w.reviewPending), p...)
			w.reviewPending = ""
			return p
		}
		if len(p) == 0 {
			return nil
		}
		w.processReviewLine(w.reviewPending)
		w.reviewPending = ""
		p = p[1:]
	}
	return nil
}

func isAURReviewEnd(line string) bool {
	line = aurOperationCounter.ReplaceAllString(line, "")
	for _, marker := range []string{
		"==> PKGBUILDs to edit?", "==> Making package:",
		":: Synchronizing package databases", ":: Parsing SRCINFO:", "Parsing SRCINFO:",
		":: Proceed with install?", ":: Proceed with installation?",
	} {
		// Diff content has a leading space, '+' or '-'; it must stay in the review.
		if strings.HasPrefix(line, marker) {
			return true
		}
	}
	return false
}

func (w *aurOutput) processReviewLine(raw string) {
	line := sanitizeTerminal(raw)
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "diff --git ") {
		pkg := w.diffPackageFromHeader(trimmed)
		if pkg != "" && pkg != w.diffPackage {
			w.diffPackage = pkg
			w.diffFile = ""
			w.renderer.DiffPackage(pkg, "")
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
	for _, pkg := range w.opts.diffPackages {
		if strings.Contains(header, "/"+pkg+"/") {
			return pkg
		}
	}
	if len(w.opts.diffPackages) == 1 {
		return w.opts.diffPackages[0]
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
	// A newline-terminated prompt was already answered (or needed no answer,
	// e.g. under --noconfirm); only unterminated ones wait for input.
	if isInlineAURPrompt(line) {
		w.installPlan = false
		return
	}

	if m := aurSelectionLine.FindStringSubmatch(line); m != nil {
		w.noteCandidate(m[1])
		w.selections = append(w.selections, line)
		if len(w.selections) > 20 {
			w.selections = w.selections[len(w.selections)-20:]
		}
		return
	}
	if m := aurDependencyList.FindStringSubmatch(line); m != nil {
		for _, pkg := range strings.Split(m[1], ",") {
			if name := packageNameFromFile(strings.TrimSpace(pkg)); name != "" {
				w.noteCandidate("aur/" + name)
			}
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
		w.noteCandidate(strings.Fields(line)[0])
		_, _ = fmt.Fprintf(w.out, "      %s\n", line)
		return
	}
	if packageName := aurPackageName(line); packageName != "" {
		w.packageName = packageName
		w.startBuild(packageName)
	}
	if phase := aurPhase(line, w.packageName); phase != "" {
		w.installPlan = false
		w.setPhase(phase)
		return
	}

	if rest, ok := strings.CutPrefix(line, "Packages ("); ok {
		w.stopPhase()
		w.closeBuild()
		w.installPlan = true
		w.installLines = []string{line}
		w.installNames = nil
		if i := strings.Index(rest, ")"); i >= 0 {
			w.addInstallNames(rest[i+1:])
		}
		return
	}
	if w.installPlan {
		if m := netUpgradeSize.FindStringSubmatch(line); m != nil {
			w.netBytes += parseSize(m[1], m[2])
		}
		if len(w.installLines) < maxInstallPlanLines {
			w.installLines = append(w.installLines, line)
		}
		// pacman wraps long package lists onto indented continuation lines.
		if !strings.HasPrefix(line, "Total ") && !strings.HasPrefix(line, "Net ") && !strings.HasPrefix(line, "::") {
			w.addInstallNames(line)
		}
	}
}

func (w *aurOutput) addInstallNames(list string) {
	for _, file := range strings.Fields(list) {
		if name := packageNameFromFile(file); name != "" {
			w.installNames = append(w.installNames, name)
		}
	}
}

// noteCandidate records a package yay intends to build. "repo/name" forms
// from selection and dependency lists are reduced to AUR names only: repo
// packages are signed by Arch and are surfaced at pacman's gate instead.
//
// Names recorded here are never cleared: if yay announced something outside
// the plan at any point, every later gate is put to the person. Missing a
// name here is not sufficient to install it unseen either — whatever gets
// built still has to pass the pacman gate's package-list check.
func (w *aurOutput) noteCandidate(qualified string) {
	repo, name, ok := strings.Cut(qualified, "/")
	if !ok {
		name, repo = qualified, "aur"
	}
	if repo != "aur" || w.opts.approved == nil || w.opts.approved[name] {
		return
	}
	w.unapproved[name] = true
}

// packageNameFromFile strips "-pkgver-pkgrel" from "name-1.2-1".
func packageNameFromFile(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], "-")
}

func parseSize(value, unit string) int64 {
	var f float64
	if _, err := fmt.Sscanf(value, "%g", &f); err != nil {
		return 0
	}
	scale := map[string]float64{"B": 1, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40}[unit]
	return int64(f * scale)
}

// isRoutineAURWarning drops warnings that are pure build chatter.
func isRoutineAURWarning(message string) bool {
	return strings.HasPrefix(message, "Using existing $srcdir/ tree")
}

// isQuietAURWarning marks warnings that are normal for AUR builds and never
// actionable: most AUR sources carry no PGP signature, and a backup entry
// missing from a package is the packager's problem. They are counted in the
// summary and printed only with --verbose.
func isQuietAURWarning(message string) bool {
	return strings.Contains(message, "Skipping verification of source file PGP signatures") ||
		strings.Contains(message, "Skipping all source file integrity checks") ||
		strings.Contains(strings.ToLower(message), "backup entry file not in package")
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

func (w *aurOutput) startBuild(name string) {
	if name == w.buildName {
		return
	}
	w.closeBuild()
	w.buildName = name
	w.buildStart = w.opts.now()
}

func (w *aurOutput) closeBuild() {
	if w.buildName == "" {
		return
	}
	w.builds[w.buildName] += w.opts.now().Sub(w.buildStart)
	w.buildName = ""
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
		w.pending = ""
		w.handlePrompt(line)
		return
	}
	if w.mode == aurOutputCompact && promptTail.MatchString(line) {
		w.armPromptTimer()
	}
}

// armPromptTimer hands unrecognised, question-shaped output to the person if
// nothing else arrives shortly: the subprocess is almost certainly blocked on
// the stdin arc owns.
func (w *aurOutput) armPromptTimer() {
	w.promptGen++
	gen := w.promptGen
	w.promptTimer = time.AfterFunc(unknownPromptDelay, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if gen != w.promptGen {
			return
		}
		line := strings.TrimSpace(sanitizeTerminal(w.pending))
		if line == "" {
			return
		}
		w.pending = ""
		w.handlePrompt(line)
	})
}

func (w *aurOutput) cancelPromptTimer() {
	if w.promptTimer != nil {
		w.promptTimer.Stop()
		w.promptTimer = nil
	}
	w.promptGen++
}

// handlePrompt answers or surfaces one prompt that is blocking yay.
func (w *aurOutput) handlePrompt(raw string) {
	w.stopPhase()
	// Selection lines describe the gate being answered now; they must not
	// resurface under a later, unrelated prompt.
	defer func() { w.selections = nil }()
	line := strings.TrimSpace(strings.TrimPrefix(raw, "::"))
	defaultYes := !strings.Contains(line, "[y/N]")
	approved := w.opts.approved != nil

	switch {
	case strings.HasPrefix(line, "Proceed with install?"):
		// yay's gate before building. The builds are exactly what arc showed
		// unless yay pulled in AUR packages the person never saw.
		unapproved := sortedKeys(w.unapproved)
		if approved && len(unapproved) == 0 {
			w.answer("y", "yay build set matches the approved plan")
			return
		}
		w.flushSelections()
		label := "Build the AUR packages above?"
		if len(unapproved) > 0 {
			w.renderer.Warning(fmt.Sprintf("yay also wants to build %s not in the approved plan: %s",
				output.Count(len(unapproved), "AUR package", "AUR packages"), strings.Join(unapproved, ", ")))
			label = "Build them too?"
		}
		w.ask(label, defaultYes)
	case strings.HasPrefix(line, "Proceed with installation?"):
		// pacman's gate. Auto-approve only a transaction made entirely of
		// approved packages (and their -debug splits).
		names := w.installNames
		lines := w.installLines
		w.installPlan, w.installNames, w.installLines = false, nil, nil
		outside := w.outsidePlan(names)
		if approved && len(names) > 0 && len(outside) == 0 {
			w.answer("y", "pacman transaction matches the approved plan")
			return
		}
		// Either arc has no approval to compare against, or it could not read
		// the package list at all. Both mean nobody has confirmed what is
		// about to be installed, so show pacman's own plan and ask.
		if !approved || len(names) == 0 {
			for _, l := range lines {
				w.nativeLine(l)
			}
			w.ask("Install the packages above?", defaultYes)
			return
		}
		w.renderer.Info(fmt.Sprintf("pacman wants to install %s not in the approved plan: %s",
			output.Count(len(outside), "package", "packages"), strings.Join(outside, ", ")))
		w.ask("Install them?", defaultYes)
	default:
		w.flushSelections()
		for _, l := range w.installLines {
			w.nativeLine(l)
		}
		w.installPlan, w.installNames, w.installLines = false, nil, nil
		_, _ = fmt.Fprintf(w.out, "  %s ", line)
		answer := w.readAnswer()
		w.renderer.EndPrompt()
		w.forward(answer)
	}
}

func (w *aurOutput) outsidePlan(names []string) []string {
	var outside []string
	for _, name := range names {
		if w.opts.approved[name] || w.opts.approved[strings.TrimSuffix(name, "-debug")] {
			continue
		}
		outside = append(outside, name)
	}
	return outside
}

// ask surfaces a gate in arc's wording and forwards the person's answer.
func (w *aurOutput) ask(label string, defaultYes bool) {
	w.renderer.Prompt(label, defaultYes)
	answer := w.readAnswer()
	w.renderer.EndPrompt()
	w.forward(answer)
}

// readAnswer reads one line from the person. A closed or failed input
// answers "n": an unanswerable gate must never default to installing.
func (w *aurOutput) readAnswer() string {
	line, err := w.opts.input.ReadString('\n')
	if err != nil && line == "" {
		return "n"
	}
	return strings.TrimRight(line, "\r\n")
}

func (w *aurOutput) answer(value, reason string) {
	_, _ = fmt.Fprintf(w.log, "\n[arc] answered %q: %s\n", value, reason)
	w.forward(value)
}

func (w *aurOutput) forward(value string) {
	_, _ = io.WriteString(w.opts.answers, value+"\n")
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

func aurPhase(line, pkg string) string {
	target := "AUR updates"
	if pkg != "" {
		target = pkg
	}
	switch {
	case strings.Contains(line, "Downloaded PKGBUILD"):
		return "fetching AUR build files…"
	case strings.Contains(line, "Retrieving sources"):
		return "fetching " + target + " sources…"
	case strings.Contains(line, "Validating source files"):
		return "verifying " + target + " sources…"
	case strings.Contains(line, "Starting prepare()"):
		return "preparing " + target + "…"
	case strings.Contains(line, "Starting check()"):
		return "checking " + target + "…"
	case strings.Contains(line, "Starting package()") || strings.Contains(line, "Creating package "):
		return "packaging " + target + "…"
	case strings.Contains(line, "Starting build()") || strings.Contains(line, "Making package:"):
		return "building " + target + "…"
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
	if isQuietAURWarning(message) {
		w.renderer.QuietWarning(message)
		return
	}
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
	_, _ = fmt.Fprintf(w.out, "    %s\n", strings.TrimSpace(line))
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
