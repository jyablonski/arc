package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestManager(t *testing.T, dryRun bool) (*Manager, Paths, []Provider) {
	t.Helper()
	root := t.TempDir()
	p := Paths{
		SkillsRoot:  filepath.Join(root, "ai", "skills"),
		RulesFile:   filepath.Join(root, "ai", "AGENTS.md"),
		ClaudeDir:   filepath.Join(root, "claude"),
		CodexDir:    filepath.Join(root, "codex"),
		CursorDir:   filepath.Join(root, "cursor", "skills-cursor"),
		OpencodeDir: filepath.Join(root, "opencode"),
	}
	if err := os.MkdirAll(p.SkillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	providers := Providers(p)
	for _, pr := range providers {
		if err := os.MkdirAll(pr.SkillsDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Config{Paths: p, Providers: providers, FS: DefaultFS(), DryRun: dryRun})
	return m, p, providers
}

func writeSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\nbody\n", name, description)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func readlink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	return target
}

func isSymlink(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func TestAdd_FromFile(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	draft := t.TempDir()
	writeSkill(t, draft, "foo", "a foo skill")
	if err := m.Add(filepath.Join(draft, "SKILL.md"), false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	canonical := filepath.Join(paths.SkillsRoot, "foo", "SKILL.md")
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("canonical SKILL.md missing: %v", err)
	}
	for _, p := range providers {
		slot := filepath.Join(p.SkillsDir, "foo")
		if !isSymlink(t, slot) {
			t.Errorf("%s: expected symlink at %s", p.Name, slot)
			continue
		}
		if got := readlink(t, slot); got != filepath.Join(paths.SkillsRoot, "foo") {
			t.Errorf("%s: symlink target %q, want %q", p.Name, got, filepath.Join(paths.SkillsRoot, "foo"))
		}
	}
}

func TestAdd_FromDirectoryPreservesSidecars(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	draft := filepath.Join(t.TempDir(), "my-draft")
	writeSkill(t, draft, "canvas", "canvas skill")
	if err := os.MkdirAll(filepath.Join(draft, "sdk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draft, "sdk", "helper.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(draft, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "canvas", "sdk", "helper.py")); err != nil {
		t.Errorf("sidecar not preserved: %v", err)
	}
}

func TestAdd_RefuseOverwriteWithoutForce(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "existing")
	draft := t.TempDir()
	writeSkill(t, draft, "foo", "new draft")
	err := m.Add(filepath.Join(draft, "SKILL.md"), false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
}

func TestAdd_ForceOverwrite(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "old")
	draft := t.TempDir()
	writeSkill(t, draft, "foo", "new")
	if err := m.Add(filepath.Join(draft, "SKILL.md"), true); err != nil {
		t.Fatalf("Add --force: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(paths.SkillsRoot, "foo", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "description: new") {
		t.Errorf("content not replaced: %s", data)
	}
}

func TestAdd_ValidationFailures(t *testing.T) {
	m, _, _ := newTestManager(t, false)
	cases := []struct {
		name    string
		setup   func(t *testing.T) string // returns path to pass to Add
		wantErr string
	}{
		{
			name: "bad filename",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				p := filepath.Join(d, "skill.md")
				_ = os.WriteFile(p, []byte("---\nname: x\ndescription: y\n---\n"), 0o644)
				return p
			},
			wantErr: "must be named SKILL.md",
		},
		{
			name: "dir missing SKILL.md",
			setup: func(t *testing.T) string {
				d := filepath.Join(t.TempDir(), "draft")
				_ = os.MkdirAll(d, 0o755)
				return d
			},
			wantErr: "contains no SKILL.md",
		},
		{
			name: "description too long",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				writeSkill(t, d, "x", strings.Repeat("y", MaxDescriptionLen+1))
				return filepath.Join(d, "SKILL.md")
			},
			wantErr: "max 1024",
		},
		{
			name: "bad name",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				writeSkill(t, d, "Bad_Name", "ok")
				return filepath.Join(d, "SKILL.md")
			},
			wantErr: "must match",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := m.Add(tc.setup(t), false)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAdd_DryRunMakesNoChanges(t *testing.T) {
	m, paths, providers := newTestManager(t, true)
	draft := t.TempDir()
	writeSkill(t, draft, "foo", "ok")
	if err := m.Add(filepath.Join(draft, "SKILL.md"), false); err != nil {
		t.Fatalf("Add dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo")); !os.IsNotExist(err) {
		t.Errorf("dry-run created canonical dir: %v", err)
	}
	for _, p := range providers {
		if _, err := os.Lstat(filepath.Join(p.SkillsDir, "foo")); !os.IsNotExist(err) {
			t.Errorf("dry-run created symlink in %s", p.Name)
		}
	}
}

func TestAddNew(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	if err := m.AddNew("my-skill"); err != nil {
		t.Fatalf("AddNew: %v", err)
	}
	canonical := filepath.Join(paths.SkillsRoot, "my-skill", "SKILL.md")
	data, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("read scaffold: %v", err)
	}
	if !strings.Contains(string(data), "name: my-skill") {
		t.Errorf("scaffold missing name: %s", data)
	}
	for _, p := range providers {
		if !isSymlink(t, filepath.Join(p.SkillsDir, "my-skill")) {
			t.Errorf("%s: no symlink for scaffolded skill", p.Name)
		}
	}
}

func TestAddNew_BadName(t *testing.T) {
	m, _, _ := newTestManager(t, false)
	err := m.AddNew("Bad Name")
	if err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("want name error, got %v", err)
	}
}

func TestLinkOne_SkipsExternalSymlink(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "ok")
	external := filepath.Join(t.TempDir(), "external-target")
	writeSkill(t, external, "foo", "external")
	mustSymlink(t, external, filepath.Join(providers[0].SkillsDir, "foo"))
	if err := m.linkAllProviders("foo"); err != nil {
		t.Fatalf("linkAllProviders: %v", err)
	}
	got := readlink(t, filepath.Join(providers[0].SkillsDir, "foo"))
	if got != external {
		t.Errorf("external symlink was altered: got %q", got)
	}
}

func TestLinkOne_SkipsRealDirInSlot(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "ok")
	realDir := filepath.Join(providers[2].SkillsDir, "foo")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.linkAllProviders("foo"); err != nil {
		t.Fatalf("linkAllProviders: %v", err)
	}
	info, err := os.Lstat(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected real dir preserved, got symlink")
	}
	for _, pidx := range []int{0, 1, 3} {
		if !isSymlink(t, filepath.Join(providers[pidx].SkillsDir, "foo")) {
			t.Errorf("%s: missing symlink", providers[pidx].Name)
		}
	}
}

func TestSync_MigratesProviderLocalSkill(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "from-claude")

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Migrated != 1 {
		t.Errorf("Migrated: got %d, want 1", res.Migrated)
	}
	if res.Conflicts != 0 {
		t.Errorf("Conflicts: got %d, want 0", res.Conflicts)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo", "SKILL.md")); err != nil {
		t.Errorf("canonical not migrated: %v", err)
	}
	if !isSymlink(t, filepath.Join(providers[0].SkillsDir, "foo")) {
		t.Errorf("claude slot should be symlink after migration")
	}
}

func TestSync_DedupesByteIdentical(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "same")
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "same")

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Deduped != 1 {
		t.Errorf("Deduped: got %d, want 1", res.Deduped)
	}
	if res.Conflicts != 0 {
		t.Errorf("Conflicts: got %d, want 0", res.Conflicts)
	}
	if !isSymlink(t, filepath.Join(providers[0].SkillsDir, "foo")) {
		t.Errorf("claude dedupe should have left a symlink")
	}
}

func TestSync_RefusesDivergent(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "canonical-version")
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "different-version")

	prev := nowUnix
	t.Cleanup(func() { nowUnix = prev })
	nowUnix = func() int64 { return 42 }

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Conflicts == 0 {
		t.Errorf("expected conflict count > 0")
	}
	backup := filepath.Join(providers[0].SkillsDir, "foo.conflict.42")
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("conflict backup missing: %v", err)
	}
}

func TestSync_RefusesMultiProviderDivergence(t *testing.T) {
	m, _, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "claude-version")
	writeSkill(t, filepath.Join(providers[1].SkillsDir, "foo"), "foo", "codex-version")

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Conflicts == 0 {
		t.Errorf("expected conflicts > 0 for multi-provider divergence")
	}
	if res.Migrated != 0 {
		t.Errorf("expected zero migrations, got %d", res.Migrated)
	}
	if isSymlink(t, filepath.Join(providers[0].SkillsDir, "foo")) {
		t.Errorf("claude should not be symlinked")
	}
	if isSymlink(t, filepath.Join(providers[1].SkillsDir, "foo")) {
		t.Errorf("codex should not be symlinked")
	}
}

func TestSync_MultiProviderIdenticalMigrates(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "same")
	writeSkill(t, filepath.Join(providers[1].SkillsDir, "foo"), "foo", "same")

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Conflicts != 0 {
		t.Errorf("expected no conflicts, got %d", res.Conflicts)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo", "SKILL.md")); err != nil {
		t.Errorf("canonical not created: %v", err)
	}
	if !isSymlink(t, filepath.Join(providers[0].SkillsDir, "foo")) {
		t.Errorf("claude not symlinked")
	}
	if !isSymlink(t, filepath.Join(providers[1].SkillsDir, "foo")) {
		t.Errorf("codex not symlinked")
	}
}

func TestSync_ForwardLinksCanonicalToMissingProviders(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Linked != len(providers) {
		t.Errorf("Linked: got %d, want %d", res.Linked, len(providers))
	}
	for _, p := range providers {
		if !isSymlink(t, filepath.Join(p.SkillsDir, "foo")) {
			t.Errorf("%s: missing symlink", p.Name)
		}
	}
}

func TestSync_PrunesDanglingSymlinks(t *testing.T) {
	m, _, providers := newTestManager(t, false)
	dangling := filepath.Join(providers[0].SkillsDir, "gone")
	mustSymlink(t, filepath.Join(t.TempDir(), "does-not-exist"), dangling)

	res, err := m.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Pruned < 1 {
		t.Errorf("Pruned: got %d, want >= 1", res.Pruned)
	}
	if _, err := os.Lstat(dangling); !os.IsNotExist(err) {
		t.Errorf("dangling symlink not removed: %v", err)
	}
}

func TestSync_SkipsExternalSymlink(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")
	external := filepath.Join(t.TempDir(), "external")
	writeSkill(t, external, "foo", "ext")
	mustSymlink(t, external, filepath.Join(providers[0].SkillsDir, "foo"))

	if _, err := m.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if readlink(t, filepath.Join(providers[0].SkillsDir, "foo")) != external {
		t.Errorf("external symlink altered")
	}
}

func TestSync_DryRunMakesNoChanges(t *testing.T) {
	m, paths, providers := newTestManager(t, true)
	writeSkill(t, filepath.Join(providers[0].SkillsDir, "foo"), "foo", "x")
	if _, err := m.Sync(); err != nil {
		t.Fatalf("Sync dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo")); !os.IsNotExist(err) {
		t.Errorf("dry-run created canonical")
	}
	info, _ := os.Lstat(filepath.Join(providers[0].SkillsDir, "foo"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("dry-run converted real dir to symlink")
	}
}

func TestList_StatusPerProvider(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")
	mustSymlink(t, filepath.Join(paths.SkillsRoot, "foo"), filepath.Join(providers[0].SkillsDir, "foo"))
	if err := os.MkdirAll(filepath.Join(providers[2].SkillsDir, "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external")
	writeSkill(t, external, "foo", "ext")
	mustSymlink(t, external, filepath.Join(providers[3].SkillsDir, "foo"))

	res, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(res.Skills))
	}
	sk := res.Skills[0]
	want := map[string]Status{
		"claude":   StatusOK,
		"codex":    StatusMissing,
		"cursor":   StatusConflict,
		"opencode": StatusExternal,
	}
	for name, st := range want {
		if sk.Providers[name] != st {
			t.Errorf("%s: got %q, want %q", name, sk.Providers[name], st)
		}
	}
}

func TestList_JSONShape(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")
	res, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"name":"foo"`, `"providers":`, `"frontmatter":`, `"canonical_path":`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in JSON: %s", want, s)
		}
	}
}

func TestList_ConflictBackupsReported(t *testing.T) {
	m, _, providers := newTestManager(t, false)
	backup := filepath.Join(providers[0].SkillsDir, "foo.conflict.123")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0].Path != backup {
		t.Errorf("expected conflict backup reported, got %+v", res.Conflicts)
	}
}

func TestValidate_AllGood(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "ok")
	issues, err := m.Validate("", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

func TestValidate_ReportsErrors(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "bar"), "foo", "ok")
	issues, err := m.Validate("", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Error, "does not match directory") {
		t.Errorf("expected name/dir mismatch issue, got %+v", issues)
	}
}

func TestValidate_FixRenamesDir(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "bar"), "foo", "ok")
	issues, err := m.Validate("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected fix to resolve all issues, got %+v", issues)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo", "SKILL.md")); err != nil {
		t.Errorf("fix did not rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "bar")); !os.IsNotExist(err) {
		t.Errorf("old dir still present: %v", err)
	}
}

func TestRemove_CleansCanonicalAndSymlinks(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")
	for _, p := range providers {
		mustSymlink(t, filepath.Join(paths.SkillsRoot, "foo"), filepath.Join(p.SkillsDir, "foo"))
	}
	if err := m.Remove("foo"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsRoot, "foo")); !os.IsNotExist(err) {
		t.Errorf("canonical not removed: %v", err)
	}
	for _, p := range providers {
		if _, err := os.Lstat(filepath.Join(p.SkillsDir, "foo")); !os.IsNotExist(err) {
			t.Errorf("%s symlink not removed: %v", p.Name, err)
		}
	}
}

func TestRemove_CanonicalMissingStillSweeps(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	mustSymlink(t, filepath.Join(paths.SkillsRoot, "foo"), filepath.Join(providers[0].SkillsDir, "foo"))
	if err := m.Remove("foo"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(providers[0].SkillsDir, "foo")); !os.IsNotExist(err) {
		t.Errorf("claude symlink not swept")
	}
}

func TestRemove_CanonicalIsSymlinkOnlyUnlinks(t *testing.T) {
	m, paths, _ := newTestManager(t, false)
	external := filepath.Join(t.TempDir(), "external")
	writeSkill(t, external, "foo", "repo-scoped")
	mustSymlink(t, external, filepath.Join(paths.SkillsRoot, "foo"))

	if err := m.Remove("foo"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(external, "SKILL.md")); err != nil {
		t.Errorf("external target should survive remove-by-symlink: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(paths.SkillsRoot, "foo")); !os.IsNotExist(err) {
		t.Errorf("canonical symlink not unlinked: %v", err)
	}
}

func TestRemove_RefusesToTouchRealFileInProviderSlot(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "foo"), "foo", "x")
	realDir := filepath.Join(providers[0].SkillsDir, "foo")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "keep.txt"), []byte("important"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove("foo"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(realDir, "keep.txt")); err != nil {
		t.Errorf("real content in provider slot was destroyed: %v", err)
	}
}

func TestPrune_RemovesOnlyDangling(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	writeSkill(t, filepath.Join(paths.SkillsRoot, "good"), "good", "x")
	mustSymlink(t, filepath.Join(paths.SkillsRoot, "good"), filepath.Join(providers[0].SkillsDir, "good"))
	mustSymlink(t, filepath.Join(paths.SkillsRoot, "dead"), filepath.Join(providers[0].SkillsDir, "dead"))
	if err := os.MkdirAll(filepath.Join(providers[0].SkillsDir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}

	n, err := m.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 prune, got %d", n)
	}
	if _, err := os.Lstat(filepath.Join(providers[0].SkillsDir, "good")); err != nil {
		t.Errorf("live symlink removed")
	}
	if _, err := os.Lstat(filepath.Join(providers[0].SkillsDir, "dead")); !os.IsNotExist(err) {
		t.Errorf("dangling symlink not pruned")
	}
	if _, err := os.Stat(filepath.Join(providers[0].SkillsDir, "real")); err != nil {
		t.Errorf("real dir was touched")
	}
}
