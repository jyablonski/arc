package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyablonski/arc/internal/shell"
	"golang.org/x/sys/unix"
)

type SudoFunc func(name string, args ...string) (string, error)

// FS mirrors os file operations; it runs sudo when NeedsSudo says the path is not writable.
type FS struct {
	Sudo SudoFunc
}

func DefaultFS() *FS {
	return &FS{Sudo: shell.RunSudo}
}

func NeedsSudo(path string) bool {
	p := path
	for {
		if _, err := os.Stat(p); err == nil {
			return unix.Access(p, unix.W_OK) != nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

func (f *FS) Symlink(target, linkPath string) error {
	if NeedsSudo(linkPath) {
		_, err := f.Sudo("ln", "-s", target, linkPath)
		return err
	}
	return os.Symlink(target, linkPath)
}

func (f *FS) MkdirAll(path string, perm os.FileMode) error {
	if NeedsSudo(path) {
		_, err := f.Sudo("mkdir", "-p", path)
		return err
	}
	return os.MkdirAll(path, perm)
}

func (f *FS) Remove(path string) error {
	if NeedsSudo(path) {
		_, err := f.Sudo("rm", path)
		return err
	}
	return os.Remove(path)
}

func (f *FS) RemoveAll(path string) error {
	if NeedsSudo(path) {
		_, err := f.Sudo("rm", "-rf", path)
		return err
	}
	return os.RemoveAll(path)
}

func (f *FS) Rename(oldpath, newpath string) error {
	if NeedsSudo(oldpath) || NeedsSudo(newpath) {
		_, err := f.Sudo("mv", oldpath, newpath)
		return err
	}
	return os.Rename(oldpath, newpath)
}

func (f *FS) CopyTree(src, dst string) error {
	if NeedsSudo(src) || NeedsSudo(dst) {
		_, err := f.Sudo("cp", "-a", src, dst)
		if err != nil {
			return err
		}
		user := os.Getenv("USER")
		if user != "" && NeedsSudo(dst) {
			if _, err := f.Sudo("chown", "-R", user+":"+user, dst); err != nil {
				return err
			}
		}
		return nil
	}
	return copyTreeNative(src, dst)
}

func (f *FS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if NeedsSudo(path) {
		return fmt.Errorf("WriteFile sudo path not supported for %s", path)
	}
	return os.WriteFile(path, data, perm)
}

func copyTreeNative(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	mode := info.Mode()

	switch {
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case mode.IsDir():
		if err := os.MkdirAll(dst, mode.Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTreeNative(
				filepath.Join(src, e.Name()),
				filepath.Join(dst, e.Name()),
			); err != nil {
				return err
			}
		}
		return nil
	case mode.IsRegular():
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	default:
		return fmt.Errorf("unsupported file type for %s", src)
	}
}

type Paths struct {
	SkillsRoot  string // default: ~/ai/skills
	RulesFile   string // default: ~/ai/AGENTS.md
	ClaudeDir   string // default: ~/.claude
	CodexDir    string // default: ~/.codex
	CursorDir   string // default: ~/.cursor/skills-cursor
	OpencodeDir string // default: ~/.config/opencode
}

// DefaultPaths reads HOME and honors env overrides:
//
//	ARC_SKILLS_ROOT         -> skills canonical root (and parent for AGENTS.md)
//	ARC_CLAUDE_DIR          -> ~/.claude
//	ARC_CODEX_DIR           -> ~/.codex
//	ARC_CURSOR_SKILLS_DIR   -> ~/.cursor/skills-cursor
//	ARC_OPENCODE_DIR        -> ~/.config/opencode
func DefaultPaths() Paths {
	home := os.Getenv("HOME")

	skillsRoot := os.Getenv("ARC_SKILLS_ROOT")
	if skillsRoot == "" {
		skillsRoot = filepath.Join(home, "ai", "skills")
	}
	rulesFile := filepath.Join(filepath.Dir(skillsRoot), "AGENTS.md")

	claude := envOr("ARC_CLAUDE_DIR", filepath.Join(home, ".claude"))
	codex := envOr("ARC_CODEX_DIR", filepath.Join(home, ".codex"))
	cursor := envOr("ARC_CURSOR_SKILLS_DIR", filepath.Join(home, ".cursor", "skills-cursor"))
	opencode := envOr("ARC_OPENCODE_DIR", filepath.Join(home, ".config", "opencode"))

	return Paths{
		SkillsRoot:  skillsRoot,
		RulesFile:   rulesFile,
		ClaudeDir:   claude,
		CodexDir:    codex,
		CursorDir:   cursor,
		OpencodeDir: opencode,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
