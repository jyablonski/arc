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
	skillsListCheck   bool
)

var (
	ErrSkillsConflict = errors.New("skills: unresolved conflicts")
	ErrSkillsDrift    = errors.New("skills: provider links out of sync")
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage shared AI/LLM skill definitions",
	Long: `Manage skills across Claude, Codex, Cursor, and opencode.

The canonical store is ~/ai/skills/<name>/SKILL.md. arc maintains symlinks
from each provider's skills directory back to it, validates frontmatter, and
never clobbers real content in provider slots.`,
}

func newManager(cmd *cobra.Command) *skills.Manager {
	return skills.New(skills.Config{DryRun: skillsDryRun, Log: stepLog(cmd)})
}

// dryRunMeta is the title-line marker for a run that changes nothing.
func dryRunMeta(dryRun bool) string {
	if dryRun {
		return "dry run"
	}
	return ""
}

// countPart is a summary fragment that is dropped when there is nothing to
// count, so a clean run's closing line stays short.
func countPart(n int, label string) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", n, label)
}

// tense picks the wording for something a real run did and a dry run only
// planned, so a dry run's closing line never claims a change.
func tense(dryRun bool, done, planned string) string {
	if dryRun {
		return planned
	}
	return done
}

var skillsAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a skill to canonical and link it into every provider",
	Long: `Promote a draft SKILL.md (file or directory containing one) into
~/ai/skills/<name>/ and symlink it into every provider whose slot is empty.

Use --new <name> to scaffold from an embedded template instead of promoting a
draft.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager(cmd)
		if skillsAddNew != "" && len(args) > 0 {
			return fmt.Errorf("--new and [path] are mutually exclusive")
		}
		if skillsAddNew == "" && len(args) != 1 {
			return fmt.Errorf("exactly one path argument is required")
		}
		output.Title("arc skills add", dryRunMeta(skillsDryRun))
		if skillsAddNew != "" {
			if err := m.AddNew(skillsAddNew); err != nil {
				return err
			}
			output.Summary(output.GlyphOK, tense(skillsDryRun, "scaffolded ", "would scaffold ")+skillsAddNew)
			return nil
		}
		if err := m.Add(args[0], skillsAddForce); err != nil {
			return err
		}
		output.Summary(output.GlyphOK, tense(skillsDryRun, "skill added", "skill would be added"))
		return nil
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
		m := newManager(cmd)
		output.Title("arc skills sync", dryRunMeta(skillsDryRun))
		res, err := m.Sync()
		if err != nil {
			return err
		}
		parts := []string{
			countPart(res.Linked, tense(skillsDryRun, "linked", "to link")),
			countPart(res.MetadataUpdated, tense(skillsDryRun, "metadata updated", "metadata to update")),
			countPart(res.Pruned, tense(skillsDryRun, "pruned", "to prune")),
		}
		if res.Conflicts > 0 {
			output.Summary(output.GlyphWarn, append(parts, output.Count(res.Conflicts, "conflict", "conflicts"))...)
			return ErrSkillsConflict
		}
		if res.Linked+res.MetadataUpdated+res.Pruned == 0 {
			parts = []string{"already in sync"}
		}
		output.Summary(output.GlyphOK, parts...)
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
		m := newManager(cmd)
		output.Title("arc skills export", dryRunMeta(skillsDryRun))
		res, err := m.Export(args[0])
		if err != nil {
			return err
		}
		parts := []string{
			fmt.Sprintf("%d %s", res.Exported, tense(skillsDryRun, "exported", "to export")),
			countPart(res.Deduped, tense(skillsDryRun, "deduped", "to dedupe")),
		}
		if res.Conflicts > 0 {
			output.Summary(output.GlyphWarn, append(parts, output.Count(res.Conflicts, "conflict", "conflicts"))...)
			return ErrSkillsConflict
		}
		output.Summary(output.GlyphOK, parts...)
		return nil
	},
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canonical skills and their per-provider status",
	Long: `Shows every canonical skill with one column per provider.

Cells: ✓ linked to canonical, · not linked yet, ≠ links somewhere else,
⚠ real files where the link belongs, ✗ dangling link. Skills present in a
provider but not in ~/ai/skills are listed as unmanaged.

Use --check for a single line and a non-zero exit when any link is out of
sync (for shell prompts and cron).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager(cmd)
		res, err := m.List()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		providers := skills.Providers(skills.DefaultPaths())
		switch {
		case jsonOut:
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(res); err != nil {
				return err
			}
		case skillsListCheck:
			fmt.Println(skills.CheckLine(output.StyleFor(os.Stdout), len(providers), res))
		default:
			skills.PrintListHuman(os.Stdout, providers, res)
		}
		if skillsListCheck && res.Issues() > 0 {
			return ErrSkillsDrift
		}
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
		m := newManager(cmd)
		var name string
		if len(args) == 1 {
			name = args[0]
		}
		output.Title("arc skills validate", dryRunMeta(skillsDryRun))
		issues, err := m.Validate(name, skillsValidateFix)
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			output.Summary(output.GlyphOK, "all skills valid")
			return nil
		}
		for _, issue := range issues {
			output.Error(fmt.Sprintf("%s: %s", issue.Skill, issue.Error))
		}
		output.Summary(output.GlyphFail, output.Count(len(issues), "validation issue", "validation issues"))
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
		m := newManager(cmd)
		output.Title("arc skills remove", dryRunMeta(skillsDryRun))
		if err := m.Remove(args[0]); err != nil {
			return err
		}
		output.Summary(output.GlyphOK, tense(skillsDryRun, "removed ", "would remove ")+args[0])
		return nil
	},
}

var skillsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove dangling symlinks from provider dirs",
	Long: `Remove symlinks in each provider skills directory whose target does not
exist.

Never touches canonical trees or real files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newManager(cmd)
		output.Title("arc skills prune", dryRunMeta(skillsDryRun))
		n, err := m.Prune()
		if err != nil {
			return err
		}
		if n == 0 {
			output.Summary(output.GlyphInfo, "no dangling symlinks")
			return nil
		}
		output.Summary(output.GlyphOK, output.Count(n, "dangling symlink", "dangling symlinks")+tense(skillsDryRun, " pruned", " to prune"))
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
	skillsListCmd.Flags().BoolVar(&skillsListCheck, "check", false, "Print one verdict line and exit non-zero on drift")
	skillsValidateCmd.Flags().BoolVar(&skillsValidateFix, "fix", false, "Auto-rename canonical dir on name/dir mismatch")

	skillsCmd.PersistentFlags().BoolVar(&skillsDryRun, "dry-run", false, "Print planned actions without modifying the filesystem")
}
