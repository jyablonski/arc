package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPaths_EnvOverrides(t *testing.T) {
	t.Setenv("HOME", "/home/fake")
	t.Setenv("ARC_SKILLS_ROOT", "/tmp/arc/skills")
	t.Setenv("ARC_CLAUDE_DIR", "/tmp/arc/claude")
	t.Setenv("ARC_CODEX_DIR", "/tmp/arc/codex")
	t.Setenv("ARC_CURSOR_SKILLS_DIR", "/tmp/arc/cursor")
	t.Setenv("ARC_OPENCODE_DIR", "/tmp/arc/opencode")

	p := DefaultPaths()
	if p.SkillsRoot != "/tmp/arc/skills" {
		t.Errorf("SkillsRoot: %s", p.SkillsRoot)
	}
	if p.RulesFile != "/tmp/arc/AGENTS.md" {
		t.Errorf("RulesFile: %s", p.RulesFile)
	}
	if p.ClaudeDir != "/tmp/arc/claude" {
		t.Errorf("ClaudeDir: %s", p.ClaudeDir)
	}
	if p.OpencodeDir != "/tmp/arc/opencode" {
		t.Errorf("OpencodeDir: %s", p.OpencodeDir)
	}
}

func TestDefaultPaths_Defaults(t *testing.T) {
	t.Setenv("HOME", "/home/fake")
	t.Setenv("ARC_SKILLS_ROOT", "")
	t.Setenv("ARC_CLAUDE_DIR", "")
	t.Setenv("ARC_CODEX_DIR", "")
	t.Setenv("ARC_CURSOR_SKILLS_DIR", "")
	t.Setenv("ARC_OPENCODE_DIR", "")

	p := DefaultPaths()
	wants := map[string]string{
		"SkillsRoot":  "/home/fake/ai/skills",
		"RulesFile":   "/home/fake/ai/AGENTS.md",
		"ClaudeDir":   "/home/fake/.claude",
		"CodexDir":    "/home/fake/.codex",
		"CursorDir":   "/home/fake/.cursor/skills-cursor",
		"OpencodeDir": "/home/fake/.config/opencode",
	}
	got := map[string]string{
		"SkillsRoot":  p.SkillsRoot,
		"RulesFile":   p.RulesFile,
		"ClaudeDir":   p.ClaudeDir,
		"CodexDir":    p.CodexDir,
		"CursorDir":   p.CursorDir,
		"OpencodeDir": p.OpencodeDir,
	}
	for k, want := range wants {
		if got[k] != want {
			t.Errorf("%s: got %q, want %q", k, got[k], want)
		}
	}
}

func TestNeedsSudo(t *testing.T) {
	dir := t.TempDir()
	if NeedsSudo(dir) {
		t.Errorf("expected NeedsSudo(%s) = false", dir)
	}
	child := filepath.Join(dir, "nonexistent", "deep", "path")
	if NeedsSudo(child) {
		t.Errorf("expected NeedsSudo(%s) = false (walks up)", child)
	}
	if os.Getuid() != 0 {
		if !NeedsSudo("/root/nonexistent") {
			t.Errorf("expected NeedsSudo(/root/nonexistent) = true for non-root user")
		}
	}
}

func TestFS_SymlinkAndRemove(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	fs := DefaultFS()
	if err := fs.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink")
	}
	if err := fs.Remove(link); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("expected link removed, got err=%v", err)
	}
}

func TestFS_CopyTree(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "dst")
	if err := DefaultFS().CopyTree(src, dst); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	for _, rel := range []string{"SKILL.md", "sub/b.txt"} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("missing %s in copied tree: %v", rel, err)
		}
	}
}
