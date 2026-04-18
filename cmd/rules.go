package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/skills"
	"github.com/spf13/cobra"
)

var rulesDryRun bool

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage shared AGENTS.md across AI providers",
	Long: `Manage the shared ~/ai/AGENTS.md rules file.

arc symlinks it into each provider rules-file location (~/.claude/CLAUDE.md,
~/.codex/AGENTS.md, ~/.config/opencode/AGENTS.md). Cursor has no rules-file
target.`,
}

var rulesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Symlink ~/ai/AGENTS.md into each provider's rules file",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := skills.New(skills.Config{DryRun: rulesDryRun})
		conflicts, err := m.SyncRules()
		if err != nil {
			return err
		}
		if conflicts > 0 {
			return fmt.Errorf("%d rules-file conflict(s)", conflicts)
		}
		output.Success("rules synced")
		return nil
	},
}

var rulesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show rules-file symlink state per provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := skills.New(skills.Config{})
		res := m.StatusRules()
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		output.Header(fmt.Sprintf("Canonical: %s", res.Canonical))
		headers := []string{"PROVIDER", "TARGET", "STATUS"}
		rows := make([][]string, 0, len(res.Providers))
		for _, p := range res.Providers {
			rows = append(rows, []string{p.Provider, p.Target, string(p.Status)})
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rulesCmd)
	rulesCmd.AddCommand(rulesSyncCmd)
	rulesCmd.AddCommand(rulesStatusCmd)

	rulesCmd.PersistentFlags().BoolVar(&rulesDryRun, "dry-run", false, "Print planned actions without modifying the filesystem")
}
