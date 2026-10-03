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
		m := skills.New(skills.Config{DryRun: rulesDryRun, Log: stepLog(cmd)})
		output.Title("arc rules sync", dryRunMeta(rulesDryRun))
		conflicts, err := m.SyncRules()
		if err != nil {
			return err
		}
		if conflicts > 0 {
			output.Summary(output.GlyphWarn, output.Count(conflicts, "conflict", "conflicts"))
			return fmt.Errorf("%d rules-file conflict(s)", conflicts)
		}
		output.Summary(output.GlyphOK, tense(rulesDryRun, "rules synced", "dry run complete"))
		return nil
	},
}

var rulesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show rules-file symlink state per provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := skills.New(skills.Config{Log: stepLog(cmd)})
		res := m.StatusRules()
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		style := output.StyleFor(os.Stdout)
		sc := &output.Screen{W: os.Stdout, Style: style, Title: "arc rules status", Meta: output.TildePath(res.Canonical)}
		grid := output.Grid{Columns: []output.Column{
			{Align: output.AlignCenter},
			{Header: "provider"},
			{Header: "status"},
			{Header: "target", Flex: true},
		}}
		drift := 0
		for _, p := range res.Providers {
			glyph := output.GlyphOK
			switch p.Status {
			case skills.StatusOK:
			case skills.StatusMissing:
				glyph = output.GlyphInfo
				drift++
			default:
				glyph = output.GlyphWarn
				drift++
			}
			grid.Rows = append(grid.Rows, []string{style.Glyph(glyph), p.Provider, string(p.Status), output.TildePath(p.Target)})
		}
		sc.Grid(grid)
		if drift > 0 {
			sc.Flush(style.Glyph(output.GlyphWarn) + " " + output.Count(drift, "provider", "providers") + " out of sync" + style.Sep() + style.Faint("arc rules sync"))
			return nil
		}
		sc.Flush(style.Glyph(output.GlyphOK) + " " + output.Count(len(res.Providers), "provider", "providers") + " in sync")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rulesCmd)
	rulesCmd.AddCommand(rulesSyncCmd)
	rulesCmd.AddCommand(rulesStatusCmd)

	rulesCmd.PersistentFlags().BoolVar(&rulesDryRun, "dry-run", false, "Print planned actions without modifying the filesystem")
}
