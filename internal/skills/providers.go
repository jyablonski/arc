package skills

import "path/filepath"

type Provider struct {
	Name      string
	SkillsDir string
	RulesFile string
	// SharedDir marks a skills dir the tool also fills with its own bundled
	// skills as real directories; only symlinks there can be user-managed.
	SharedDir bool
}

func Providers(p Paths) []Provider {
	return []Provider{
		{
			Name:      "claude",
			SkillsDir: filepath.Join(p.ClaudeDir, "skills"),
			RulesFile: filepath.Join(p.ClaudeDir, "CLAUDE.md"),
		},
		{
			Name:      "codex",
			SkillsDir: filepath.Join(p.CodexDir, "skills"),
			RulesFile: filepath.Join(p.CodexDir, "AGENTS.md"),
		},
		{
			Name:      "cursor",
			SkillsDir: p.CursorDir,
			RulesFile: "",
			SharedDir: true,
		},
		{
			Name:      "opencode",
			SkillsDir: filepath.Join(p.OpencodeDir, "skills"),
			RulesFile: filepath.Join(p.OpencodeDir, "AGENTS.md"),
		},
	}
}
