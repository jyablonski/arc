package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/skills"
	"github.com/spf13/cobra"
)

var (
	skillsAddForce    bool
	skillsAddNew      string
	skillsValidateFix bool
	skillsDryRun      bool
)

var ErrSkillsConflict = errors.New("skills: unresolved conflicts")

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage shared AI/LLM skill definitions",
	Long: `Manage skills across Claude, Codex, Cursor, and opencode.

The canonical store is ~/ai/skills/<name>/SKILL.md. arc maintains symlinks
from each provider's skills directory back to it, validates frontmatter, and
never clobbers real content in provider slots.`,
}

func newManager() *skills.Manager {
	return skills.New(skills.Config{DryRun: skillsDryRun})
}

var skillsAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a skill to canonical and link it into every provider",
	Long: `Promote a draft SKILL.md (file or directory containing one) into
~/ai/skills/<name>/ and symlink it into every provider whose slot is empty.

Use --new <name> to scaffold from an embedded template instead of promoting a
draft.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager()
		if skillsAddNew != "" {
			if len(args) > 0 {
				return fmt.Errorf("--new and [path] are mutually exclusive")
			}
			return m.AddNew(skillsAddNew)
		}
		if len(args) != 1 {
			return fmt.Errorf("exactly one path argument is required")
		}
		return m.Add(args[0], skillsAddForce)
	},
}

var skillsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Forward-link canonical skills into every provider",
	Long: `Symlink every canonical skill under ~/ai/skills into every provider and
prune dangling symlinks. Frontmatter disable-model-invocation values are also
translated into Codex agents/openai.yaml policy. Real files in provider slots
are never touched.

Exits with a non-zero status if any conflict is unresolved.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager()
		res, err := m.Sync()
		if err != nil {
			return err
		}
		output.Header("Sync summary")
		output.Info(fmt.Sprintf("linked: %d, metadata updated: %d, pruned: %d, conflicts: %d",
			res.Linked, res.MetadataUpdated, res.Pruned, res.Conflicts))
		if res.Conflicts > 0 {
			return ErrSkillsConflict
		}
		return nil
	},
}

var skillsExportCmd = &cobra.Command{
	Use:   "export <parent_folder>",
	Short: "Copy canonical skill directories into a parent folder",
	Long: `Copy every canonical skill under ~/ai/skills into <parent_folder>.

Byte-identical destination copies are deduped. Divergent destination copies are
reported as conflicts and never overwritten.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one parent_folder argument is required")
		}
		m := newManager()
		res, err := m.Export(args[0])
		if err != nil {
			return err
		}
		output.Header("Export summary")
		output.Info(fmt.Sprintf("exported: %d, deduped: %d, conflicts: %d",
			res.Exported, res.Deduped, res.Conflicts))
		if res.Conflicts > 0 {
			return ErrSkillsConflict
		}
		return nil
	},
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canonical skills and their per-provider status",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager()
		res, err := m.List()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		skills.PrintListHuman(os.Stdout, skills.Providers(skills.DefaultPaths()), res)
		return nil
	},
}

var skillsValidateCmd = &cobra.Command{
	Use:   "validate [name]",
	Short: "Check every canonical skill against the frontmatter schema",
	Long: `Runs the six-rule schema on every canonical skill, or on one skill when
a name argument is given.

Use --fix to rename the canonical directory when it disagrees with
frontmatter.name, then run arc skills sync to refresh symlinks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager()
		var name string
		if len(args) == 1 {
			name = args[0]
		}
		issues, err := m.Validate(name, skillsValidateFix)
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			output.Success("all skills valid")
			return nil
		}
		for _, issue := range issues {
			output.Error(fmt.Sprintf("%s: %s", issue.Skill, issue.Error))
		}
		return fmt.Errorf("%d validation issue(s)", len(issues))
	},
}

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a skill from canonical and sweep provider symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one skill name is required")
		}
		m := newManager()
		return m.Remove(args[0])
	},
}

var skillsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove dangling symlinks from provider dirs",
	Long: `Remove symlinks in each provider skills directory whose target does not
exist.

Never touches canonical trees or real files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager()
		n, err := m.Prune()
		if err != nil {
			return err
		}
		output.Info(fmt.Sprintf("pruned %d dangling symlink(s)", n))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsAddCmd)
	skillsCmd.AddCommand(skillsSyncCmd)
	skillsCmd.AddCommand(skillsExportCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsValidateCmd)
	skillsCmd.AddCommand(skillsRemoveCmd)
	skillsCmd.AddCommand(skillsPruneCmd)

	skillsAddCmd.Flags().BoolVar(&skillsAddForce, "force", false, "Overwrite existing canonical skill")
	skillsAddCmd.Flags().StringVar(&skillsAddNew, "new", "", "Scaffold a new skill with this name instead of promoting a draft")
	skillsValidateCmd.Flags().BoolVar(&skillsValidateFix, "fix", false, "Auto-rename canonical dir on name/dir mismatch")

	skillsCmd.PersistentFlags().BoolVar(&skillsDryRun, "dry-run", false, "Print planned actions without modifying the filesystem")
}
