package skills

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/jyablonski/arc/internal/filemode"
	"github.com/jyablonski/arc/internal/output"
)

//go:embed template/SKILL.md.tmpl
var embeddedTemplates embed.FS

type Status string

const (
	StatusOK       Status = "ok"
	StatusMissing  Status = "missing"
	StatusConflict Status = "conflict"
	StatusExternal Status = "external"
	StatusDangling Status = "dangling"
)

type SkillEntry struct {
	Name          string            `json:"name"`
	CanonicalPath string            `json:"canonical_path"`
	Providers     map[string]Status `json:"providers"`
	Frontmatter   Frontmatter       `json:"frontmatter"`
}

type ConflictBackup struct {
	Provider string `json:"provider"`
	Path     string `json:"path"`
}

type ListResult struct {
	Skills    []SkillEntry     `json:"skills"`
	Conflicts []ConflictBackup `json:"conflicts"`
}

type ValidationIssue struct {
	Skill string `json:"skill"`
	Path  string `json:"path"`
	Error string `json:"error"`
}

type Config struct {
	Paths     Paths
	Providers []Provider
	FS        *FS
	DryRun    bool
}

type Manager struct {
	paths     Paths
	providers []Provider
	fs        *FS
	dryRun    bool
}

func New(c Config) *Manager {
	if c.Paths.SkillsRoot == "" {
		c.Paths = DefaultPaths()
	}
	if c.Providers == nil {
		c.Providers = Providers(c.Paths)
	}
	if c.FS == nil {
		c.FS = DefaultFS()
	}
	return &Manager{
		paths:     c.Paths,
		providers: c.Providers,
		fs:        c.FS,
		dryRun:    c.DryRun,
	}
}

func (m *Manager) announce(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if m.dryRun {
		output.Info("would " + msg)
	} else {
		output.Info(msg)
	}
}

func (m *Manager) mkdirAll(path string, perm os.FileMode) error {
	if m.dryRun {
		return nil
	}
	return m.fs.MkdirAll(path, perm)
}

func (m *Manager) Add(srcPath string, force bool) error {
	info, err := os.Lstat(srcPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}

	var skillMdPath, moveSrc string
	isDir := info.IsDir()

	if isDir {
		skillMdPath = filepath.Join(srcPath, SkillFilename)
		if _, err := os.Stat(skillMdPath); err != nil {
			return fmt.Errorf("directory %s contains no %s", srcPath, SkillFilename)
		}
		moveSrc = srcPath
	} else {
		if filepath.Base(srcPath) != SkillFilename {
			return fmt.Errorf("file must be named %s, got %s", SkillFilename, filepath.Base(srcPath))
		}
		skillMdPath = srcPath
		moveSrc = srcPath
	}

	fm, err := Parse(skillMdPath)
	if err != nil {
		return err
	}
	if err := Validate(fm, ""); err != nil {
		return err
	}

	destDir := filepath.Join(m.paths.SkillsRoot, fm.Name)
	if _, err := os.Stat(destDir); err == nil {
		if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", destDir)
		}
		m.announce("remove existing canonical %s", destDir)
		if !m.dryRun {
			if err := m.fs.RemoveAll(destDir); err != nil {
				return fmt.Errorf("remove %s: %w", destDir, err)
			}
		}
	}

	if err := m.mkdirAll(m.paths.SkillsRoot, filemode.Dir); err != nil {
		return fmt.Errorf("mkdir %s: %w", m.paths.SkillsRoot, err)
	}

	if isDir {
		m.announce("move %s -> %s", moveSrc, destDir)
		if !m.dryRun {
			if err := m.moveOrCopy(moveSrc, destDir); err != nil {
				return err
			}
		}
	} else {
		if err := m.mkdirAll(destDir, filemode.Dir); err != nil {
			return fmt.Errorf("mkdir %s: %w", destDir, err)
		}
		destFile := filepath.Join(destDir, SkillFilename)
		m.announce("move %s -> %s", moveSrc, destFile)
		if !m.dryRun {
			if err := m.moveOrCopy(moveSrc, destFile); err != nil {
				return err
			}
		}
	}

	return m.linkAllProviders(fm.Name)
}

func (m *Manager) AddNew(name string) error {
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("name %q must match %s", name, nameRegex.String())
	}
	destDir := filepath.Join(m.paths.SkillsRoot, name)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("%s already exists", destDir)
	}

	tmpl, err := template.ParseFS(embeddedTemplates, "template/SKILL.md.tmpl")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Name string }{Name: name}); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	m.announce("create skill scaffold %s", destDir)
	if !m.dryRun {
		if err := m.mkdirAll(destDir, filemode.Dir); err != nil {
			return fmt.Errorf("mkdir %s: %w", destDir, err)
		}
		if err := m.fs.WriteFile(filepath.Join(destDir, SkillFilename), buf.Bytes(), filemode.File); err != nil {
			return fmt.Errorf("write SKILL.md: %w", err)
		}
	}

	return m.linkAllProviders(name)
}

func (m *Manager) linkAllProviders(name string) error {
	canonical := filepath.Join(m.paths.SkillsRoot, name)
	for _, p := range m.providers {
		if err := m.linkOne(p, name, canonical); err != nil {
			output.Warning(fmt.Sprintf("%s: %v", p.Name, err))
		}
	}
	return nil
}

func (m *Manager) linkOne(p Provider, name, canonical string) error {
	slot := filepath.Join(p.SkillsDir, name)
	info, err := os.Lstat(slot)
	if err == nil {
		return m.reconcileExistingSlot(p, slot, info, canonical)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("lstat %s: %w", slot, err)
	}
	if err := m.mkdirAll(p.SkillsDir, filemode.Dir); err != nil {
		return fmt.Errorf("mkdir %s: %w", p.SkillsDir, err)
	}
	m.announce("create symlink %s -> %s", slot, canonical)
	if m.dryRun {
		return nil
	}
	return m.fs.Symlink(canonical, slot)
}

func (m *Manager) reconcileExistingSlot(p Provider, slot string, info os.FileInfo, canonical string) error {
	if info.Mode()&os.ModeSymlink != 0 {
		target, rerr := os.Readlink(slot)
		if rerr != nil {
			return fmt.Errorf("readlink %s: %w", slot, rerr)
		}
		resolvedTarget := target
		if !filepath.IsAbs(resolvedTarget) {
			resolvedTarget = filepath.Join(filepath.Dir(slot), target)
		}
		canonicalAbs, _ := filepath.Abs(canonical)
		targetAbs, _ := filepath.Abs(resolvedTarget)
		if targetAbs == canonicalAbs {
			output.Info(fmt.Sprintf("ok: %s -> %s", slot, canonical))
			return nil
		}
		if _, sterr := os.Stat(resolvedTarget); os.IsNotExist(sterr) {
			m.announce("replace dangling symlink %s (was -> %s)", slot, target)
			if m.dryRun {
				return nil
			}
			if err := m.fs.Remove(slot); err != nil {
				return err
			}
			return m.fs.Symlink(canonical, slot)
		}
		output.Info(fmt.Sprintf("skip (external symlink): %s -> %s", slot, target))
		return nil
	}
	output.Warning(fmt.Sprintf("conflict (real %s in slot): %s (manual review)", fileKind(info), slot))
	return nil
}

func (m *Manager) moveOrCopy(src, dst string) error {
	if err := m.fs.Rename(src, dst); err == nil {
		return nil
	}
	if err := m.fs.CopyTree(src, dst); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := m.fs.RemoveAll(src); err != nil {
		return fmt.Errorf("remove %s: %w", src, err)
	}
	return nil
}

type SyncResult struct {
	Linked    int
	Pruned    int
	Conflicts int
}

func (m *Manager) Sync() (SyncResult, error) {
	var res SyncResult

	if err := m.mkdirAll(m.paths.SkillsRoot, filemode.Dir); err != nil {
		return res, fmt.Errorf("mkdir %s: %w", m.paths.SkillsRoot, err)
	}

	canonicalNames, err := m.canonicalSkills()
	if err != nil {
		return res, err
	}
	for _, name := range canonicalNames {
		canonical := filepath.Join(m.paths.SkillsRoot, name)
		for _, p := range m.providers {
			slot := filepath.Join(p.SkillsDir, name)
			info, lerr := os.Lstat(slot)
			if lerr != nil {
				if !os.IsNotExist(lerr) {
					output.Warning(fmt.Sprintf("lstat %s: %v", slot, lerr))
					continue
				}
				if err := m.mkdirAll(p.SkillsDir, filemode.Dir); err != nil {
					output.Warning(fmt.Sprintf("mkdir %s: %v", p.SkillsDir, err))
					continue
				}
				m.announce("create symlink %s -> %s", slot, canonical)
				if !m.dryRun {
					if err := m.fs.Symlink(canonical, slot); err != nil {
						output.Warning(fmt.Sprintf("symlink %s: %v", slot, err))
						continue
					}
				}
				res.Linked++
				continue
			}
			if err := m.reconcileExistingSlot(p, slot, info, canonical); err != nil {
				output.Warning(fmt.Sprintf("%s: %v", slot, err))
			}
			if info.Mode()&os.ModeSymlink == 0 {
				res.Conflicts++
			}
		}
	}

	pruned, err := m.pruneProviders()
	if err == nil {
		res.Pruned = pruned
	}

	return res, nil
}

type ExportResult struct {
	Exported  int
	Deduped   int
	Conflicts int
}

func (m *Manager) Export(parentFolder string) (ExportResult, error) {
	var res ExportResult
	if parentFolder == "" {
		return res, fmt.Errorf("parent folder is required")
	}
	if err := m.mkdirAll(parentFolder, filemode.Dir); err != nil {
		return res, fmt.Errorf("mkdir %s: %w", parentFolder, err)
	}
	info, err := os.Stat(parentFolder)
	if err != nil {
		if m.dryRun && os.IsNotExist(err) {
			names, err := m.canonicalSkills()
			if err != nil {
				return res, err
			}
			for _, name := range names {
				if err := m.exportOne(name, parentFolder, &res); err != nil {
					output.Warning(fmt.Sprintf("export %s: %v", name, err))
				}
			}
			return res, nil
		}
		return res, fmt.Errorf("stat %s: %w", parentFolder, err)
	}
	if !info.IsDir() {
		return res, fmt.Errorf("%s is not a directory", parentFolder)
	}
	names, err := m.canonicalSkills()
	if err != nil {
		return res, err
	}
	for _, name := range names {
		if err := m.exportOne(name, parentFolder, &res); err != nil {
			output.Warning(fmt.Sprintf("export %s: %v", name, err))
		}
	}
	return res, nil
}

func (m *Manager) exportOne(name, parentFolder string, res *ExportResult) error {
	src := filepath.Join(m.paths.SkillsRoot, name)
	dest := filepath.Join(parentFolder, name)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		m.announce("export %s -> %s", src, dest)
		if !m.dryRun {
			if err := m.fs.CopyTree(src, dest); err != nil {
				return err
			}
		}
		res.Exported++
		return nil
	}
	same, err := dirsEqual(src, dest)
	if err != nil {
		return err
	}
	if same {
		output.Info(fmt.Sprintf("dedupe %s (byte-identical to canonical)", dest))
		res.Deduped++
		return nil
	}
	output.Warning(fmt.Sprintf("conflict: %s differs from canonical; leaving %s unchanged", name, dest))
	res.Conflicts++
	return nil
}

func (m *Manager) pruneProviders() (int, error) {
	var count int
	for _, p := range m.providers {
		entries, err := os.ReadDir(p.SkillsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			slot := filepath.Join(p.SkillsDir, e.Name())
			info, err := os.Lstat(slot)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if _, err := os.Stat(slot); err == nil {
				continue
			}
			m.announce("prune dangling symlink %s", slot)
			if !m.dryRun {
				if err := m.fs.Remove(slot); err != nil {
					output.Warning(fmt.Sprintf("prune %s: %v", slot, err))
					continue
				}
			}
			count++
		}
	}
	return count, nil
}

func (m *Manager) Prune() (int, error) {
	return m.pruneProviders()
}

func (m *Manager) List() (ListResult, error) {
	var res ListResult

	names, err := m.canonicalSkills()
	if err != nil {
		return res, err
	}
	for _, name := range names {
		canonical := filepath.Join(m.paths.SkillsRoot, name)
		entry := SkillEntry{
			Name:          name,
			CanonicalPath: canonical,
			Providers:     map[string]Status{},
		}
		if fm, err := Parse(filepath.Join(canonical, SkillFilename)); err == nil {
			entry.Frontmatter = fm
		}
		for _, p := range m.providers {
			entry.Providers[p.Name] = slotStatus(filepath.Join(p.SkillsDir, name), canonical)
		}
		res.Skills = append(res.Skills, entry)
	}

	res.Conflicts = m.conflictBackups()
	return res, nil
}

func slotStatus(slot, canonical string) Status {
	info, err := os.Lstat(slot)
	if err != nil {
		return StatusMissing
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return StatusConflict
	}
	target, err := os.Readlink(slot)
	if err != nil {
		return StatusConflict
	}
	resolved := target
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(slot), target)
	}
	canonicalAbs, _ := filepath.Abs(canonical)
	targetAbs, _ := filepath.Abs(resolved)
	if targetAbs == canonicalAbs {
		if _, err := os.Stat(resolved); err != nil {
			return StatusDangling
		}
		return StatusOK
	}
	if _, err := os.Stat(resolved); err != nil {
		return StatusDangling
	}
	return StatusExternal
}

var conflictRe = regexp.MustCompile(`\.conflict\.\d+$`)

func (m *Manager) conflictBackups() []ConflictBackup {
	var out []ConflictBackup
	for _, p := range m.providers {
		entries, err := os.ReadDir(p.SkillsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if conflictRe.MatchString(e.Name()) {
				out = append(out, ConflictBackup{
					Provider: p.Name,
					Path:     filepath.Join(p.SkillsDir, e.Name()),
				})
			}
		}
	}
	return out
}

func (m *Manager) Validate(name string, fix bool) ([]ValidationIssue, error) {
	var issues []ValidationIssue

	names, err := m.canonicalSkills()
	if err != nil {
		return nil, err
	}
	if name != "" {
		if !contains(names, name) {
			return nil, fmt.Errorf("skill %q not found in %s", name, m.paths.SkillsRoot)
		}
		names = []string{name}
	}

	for _, n := range names {
		canonical := filepath.Join(m.paths.SkillsRoot, n)
		skillMd := filepath.Join(canonical, SkillFilename)
		fm, err := Parse(skillMd)
		if err != nil {
			issues = append(issues, ValidationIssue{Skill: n, Path: skillMd, Error: err.Error()})
			continue
		}
		verr := Validate(fm, n)
		if verr == nil {
			continue
		}
		if fix && fm.Name != "" && fm.Name != n && nameRegex.MatchString(fm.Name) {
			newPath := filepath.Join(m.paths.SkillsRoot, fm.Name)
			if _, err := os.Stat(newPath); err == nil {
				issues = append(issues, ValidationIssue{
					Skill: n, Path: skillMd,
					Error: fmt.Sprintf("cannot fix: %s already exists", newPath),
				})
				continue
			}
			m.announce("rename %s -> %s (fix name/dir mismatch)", canonical, newPath)
			if !m.dryRun {
				if err := m.fs.Rename(canonical, newPath); err != nil {
					issues = append(issues, ValidationIssue{
						Skill: n, Path: skillMd,
						Error: fmt.Sprintf("rename failed: %v", err),
					})
					continue
				}
				output.Info("run `arc skills sync` to refresh provider symlinks")
			}
			continue
		}
		issues = append(issues, ValidationIssue{Skill: n, Path: skillMd, Error: verr.Error()})
	}
	return issues, nil
}

func (m *Manager) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	canonical := filepath.Join(m.paths.SkillsRoot, name)
	info, err := os.Lstat(canonical)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		m.announce("unlink canonical symlink %s", canonical)
		if !m.dryRun {
			if err := m.fs.Remove(canonical); err != nil {
				return fmt.Errorf("remove %s: %w", canonical, err)
			}
		}
	case err == nil:
		m.announce("remove canonical %s", canonical)
		if !m.dryRun {
			if err := m.fs.RemoveAll(canonical); err != nil {
				return fmt.Errorf("remove %s: %w", canonical, err)
			}
		}
	case os.IsNotExist(err):
		output.Info(fmt.Sprintf("canonical %s already missing; sweeping providers", canonical))
	default:
		return fmt.Errorf("stat %s: %w", canonical, err)
	}

	for _, p := range m.providers {
		slot := filepath.Join(p.SkillsDir, name)
		sInfo, sErr := os.Lstat(slot)
		if sErr != nil {
			continue
		}
		if sInfo.Mode()&os.ModeSymlink == 0 {
			output.Warning(fmt.Sprintf("conflict: %s is a real %s; manual review", slot, fileKind(sInfo)))
			continue
		}
		m.announce("unlink %s (%s)", slot, p.Name)
		if !m.dryRun {
			if err := m.fs.Remove(slot); err != nil {
				output.Warning(fmt.Sprintf("unlink %s: %v", slot, err))
			}
		}
	}
	return nil
}

func (m *Manager) canonicalSkills() ([]string, error) {
	entries, err := os.ReadDir(m.paths.SkillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", m.paths.SkillsRoot, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !hasSkillFile(filepath.Join(m.paths.SkillsRoot, e.Name())) {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

func hasSkillFile(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, SkillFilename))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func dirsEqual(a, b string) (bool, error) {
	aHashes, err := hashTree(a)
	if err != nil {
		return false, err
	}
	bHashes, err := hashTree(b)
	if err != nil {
		return false, err
	}
	if len(aHashes) != len(bHashes) {
		return false, nil
	}
	for name, ah := range aHashes {
		bh, ok := bHashes[name]
		if !ok || ah != bh {
			return false, nil
		}
	}
	return true, nil
}

func hashTree(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, cerr := io.Copy(h, f)
		closeErr := f.Close()
		if cerr != nil {
			return cerr
		}
		if closeErr != nil {
			return closeErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = fmt.Sprintf("%x", h.Sum(nil))
		return nil
	})
	return out, err
}

func fileKind(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	return "file"
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

func PrintListHuman(w io.Writer, providers []Provider, res ListResult) {
	if len(res.Skills) == 0 {
		_, _ = fmt.Fprintln(w, "no skills found in canonical dir")
		return
	}
	headers := []string{"NAME", "CANONICAL"}
	for _, p := range providers {
		headers = append(headers, strings.ToUpper(p.Name))
	}
	rows := make([][]string, 0, len(res.Skills))
	for _, e := range res.Skills {
		row := []string{e.Name, e.CanonicalPath}
		for _, p := range providers {
			row = append(row, string(e.Providers[p.Name]))
		}
		rows = append(rows, row)
	}
	output.Table(headers, rows)
	if len(res.Conflicts) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Conflict backups (manual review):")
		for _, c := range res.Conflicts {
			_, _ = fmt.Fprintf(w, "  %s (%s)\n", c.Path, c.Provider)
		}
	}
}
