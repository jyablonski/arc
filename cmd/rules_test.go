package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func resetRulesFlags() { rulesDryRun = false }

func TestRulesSyncCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetRulesFlags()
	writeFile(t, filepath.Join(root, "ai", "AGENTS.md"), "shared\n")

	if err := rulesSyncCmd.RunE(rulesSyncCmd, []string{}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "claude", "CLAUDE.md")); err != nil {
		t.Errorf("claude CLAUDE.md not created: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "codex", "AGENTS.md")); err != nil {
		t.Errorf("codex AGENTS.md not created: %v", err)
	}
}

func TestRulesSyncCmd_CanonicalMissing(t *testing.T) {
	setupSkillsEnv(t)
	defer resetRulesFlags()
	if err := rulesSyncCmd.RunE(rulesSyncCmd, []string{}); err == nil {
		t.Fatal("expected error when canonical AGENTS.md missing")
	}
}

func TestRulesStatusCmd(t *testing.T) {
	root := setupSkillsEnv(t)
	defer resetRulesFlags()
	writeFile(t, filepath.Join(root, "ai", "AGENTS.md"), "x\n")
	if err := rulesStatusCmd.RunE(rulesStatusCmd, []string{}); err != nil {
		t.Errorf("status: %v", err)
	}
}
