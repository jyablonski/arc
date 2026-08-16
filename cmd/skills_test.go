package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
)

func setupSkillsEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("ARC_SKILLS_ROOT", filepath.Join(root, "ai", "skills"))
	t.Setenv("ARC_CLAUDE_DIR", filepath.Join(root, "claude"))
	t.Setenv("ARC_CODEX_DIR", filepath.Join(root, "codex"))
	t.Setenv("ARC_CURSOR_SKILLS_DIR", filepath.Join(root, "cursor", "skills-cursor"))
	t.Setenv("ARC_OPENCODE_DIR", filepath.Join(root, "opencode"))
	if err := os.MkdirAll(filepath.Join(root, "ai"), filemode.Dir); err != nil {
		t.Fatal(err)
	}
	return root
}

func resetSkillsFlags() {
	skillsAddForce = false
	skillsAddNew = ""
	skillsValidateFix = false
	skillsDryRun = false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), filemode.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), filemode.File); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsAddCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()

	draft := filepath.Join(t.TempDir(), "SKILL.md")
	writeFile(t, draft, "---\nname: foo\ndescription: ok\n---\n")

	if err := skillsAddCmd.RunE(skillsAddCmd, []string{draft}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "skills", "foo", "SKILL.md")); err != nil {
		t.Errorf("canonical missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "claude", "skills", "foo")); err != nil {
		t.Errorf("claude symlink missing: %v", err)
	}
}

func TestSkillsAddNew(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()

	skillsAddNew = "my-new-skill"
	if err := skillsAddCmd.RunE(skillsAddCmd, []string{}); err != nil {
		t.Fatalf("add --new: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "skills", "my-new-skill", "SKILL.md")); err != nil {
		t.Errorf("scaffold missing: %v", err)
	}
}

func TestSkillsAddNewMutuallyExclusive(t *testing.T) {
	setupSkillsEnv(t)
	defer resetSkillsFlags()

	skillsAddNew = "x"
	err := skillsAddCmd.RunE(skillsAddCmd, []string{"/tmp/foo"})
	if err == nil {
		t.Fatal("expected error for --new + path")
	}
}

func TestSkillsSyncCmdReturnsErrOnConflict(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()

	writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: canonical\n---\n")
	writeFile(t, filepath.Join(root, "claude", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: divergent\n---\n")

	err := skillsSyncCmd.RunE(skillsSyncCmd, []string{})
	if !errors.Is(err, ErrSkillsConflict) {
		t.Errorf("expected ErrSkillsConflict, got %v", err)
	}
}

func TestSkillsSyncCmdSuccess(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: ok\ndisable-model-invocation: true\n---\n")

	if err := skillsSyncCmd.RunE(skillsSyncCmd, []string{}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "codex", "skills", "foo")); err != nil {
		t.Errorf("codex symlink missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "ai", "skills", "foo", "agents", "openai.yaml"))
	if err != nil {
		t.Fatalf("Codex metadata missing: %v", err)
	}
	if !strings.Contains(string(data), "allow_implicit_invocation: false") {
		t.Errorf("unexpected Codex metadata: %s", data)
	}
}

func TestSkillsExportCmdRequiresArg(t *testing.T) {
	setupSkillsEnv(t)
	defer resetSkillsFlags()
	if err := skillsExportCmd.RunE(skillsExportCmd, []string{}); err == nil {
		t.Fatal("expected error for missing parent_folder")
	}
}

func TestSkillsExportCmdSuccess(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: ok\n---\n")
	dest := filepath.Join(root, "tester")

	if err := skillsExportCmd.RunE(skillsExportCmd, []string{dest}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "foo", "SKILL.md")); err != nil {
		t.Errorf("exported skill missing: %v", err)
	}
}

func TestSkillsExportCmdReturnsErrOnConflict(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: canonical\n---\n")
	dest := filepath.Join(root, "tester")
	writeFile(t, filepath.Join(dest, "foo", "SKILL.md"),
		"---\nname: foo\ndescription: divergent\n---\n")

	err := skillsExportCmd.RunE(skillsExportCmd, []string{dest})
	if !errors.Is(err, ErrSkillsConflict) {
		t.Errorf("expected ErrSkillsConflict, got %v", err)
	}
}

func TestSkillsValidateCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	writeFile(t, filepath.Join(root, "ai", "skills", "bar", "SKILL.md"),
		"---\nname: foo\ndescription: ok\n---\n")

	err := skillsValidateCmd.RunE(skillsValidateCmd, []string{})
	if err == nil {
		t.Fatal("expected validation error")
	}

	skillsValidateFix = true
	defer resetSkillsFlags()
	if err := skillsValidateCmd.RunE(skillsValidateCmd, []string{}); err != nil {
		t.Errorf("validate --fix: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "skills", "foo", "SKILL.md")); err != nil {
		t.Errorf("fix did not rename: %v", err)
	}
}

func TestSkillsRemoveCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	writeFile(t, filepath.Join(root, "ai", "skills", "foo", "SKILL.md"),
		"---\nname: foo\ndescription: ok\n---\n")

	if err := skillsRemoveCmd.RunE(skillsRemoveCmd, []string{"foo"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "skills", "foo")); !os.IsNotExist(err) {
		t.Errorf("canonical not removed: %v", err)
	}
}

func TestSkillsRemoveCmdRequiresArg(t *testing.T) {
	setupSkillsEnv(t)
	defer resetSkillsFlags()
	if err := skillsRemoveCmd.RunE(skillsRemoveCmd, []string{}); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestSkillsPruneCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetSkillsFlags()
	claude := filepath.Join(root, "claude", "skills")
	if err := os.MkdirAll(claude, filemode.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), filepath.Join(claude, "gone")); err != nil {
		t.Fatal(err)
	}
	if err := skillsPruneCmd.RunE(skillsPruneCmd, []string{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(claude, "gone")); !os.IsNotExist(err) {
		t.Errorf("dangling not pruned")
	}
}

func TestSkillsListCmdEmpty(t *testing.T) {
	setupSkillsEnv(t)
	defer resetSkillsFlags()
	if err := skillsListCmd.RunE(skillsListCmd, []string{}); err != nil {
		t.Errorf("list empty: %v", err)
	}
}
