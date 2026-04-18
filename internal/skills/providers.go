package skills

import "path/filepath"

type Provider struct {
	Name      string
	SkillsDir string
	RulesFile string
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
		},
		{
			Name:      "opencode",
			SkillsDir: filepath.Join(p.OpencodeDir, "skills"),
			RulesFile: filepath.Join(p.OpencodeDir, "AGENTS.md"),
		},
	}
}
