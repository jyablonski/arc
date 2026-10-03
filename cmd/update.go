package cmd

import (
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/arcerrs"
	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/selfupdate"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var (
	updateNoAUR   bool
	updateNoCache bool
	updateYes     bool
	updateLog     bool
	updateVerbose bool
	updateDiff    bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Run updates for arc, the system, or tools",
	Long: `Subcommands pick what to upgrade:

  self   — Fetch the latest arc release from GitHub and replace this binary.

  system — Run the host package-manager update workflow. Linux uses pacman/yay;
           macOS uses Homebrew.

  uv     — Run uv self update.`,
}

var updateSelfCmd = &cobra.Command{
	Use:   "self",
	Short: "Update arc to the latest version",
	Long:  `Check for the latest release on GitHub and update arc to that version if available.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return selfupdate.New().Upgrade(os.Stdout, version)
	},
}

var updateSystemCmd = &cobra.Command{
	Use:   "system",
	Short: "Run system package updates",
	Long: `Update the system packages. On Linux, arc renders and gates the resolved pacman
transaction, verifies installed versions, optionally updates AUR packages with
yay, and optionally cleans the package cache.

Before yay runs, arc triages pending AUR updates for takeover signals, scans
changed package files for high-signal patterns, and classifies each package's
build-file change against the last trusted snapshot. Routine bumps (pkgver,
checksums, metadata) print as one line; anything touching sources, build
functions, install hooks, or new dependencies shows its diff automatically.
Answer d at the AUR prompt (or pass --diff) to see every diff. arc then asks
once per section and answers yay's and pacman's own prompts itself only while
the transaction matches what you approved; anything else is put to you.

Routine warnings (e.g. AUR sources without PGP signatures) are counted in the
closing summary; use -v to print them. Use --log to save complete raw output
under Arc's state directory. macOS runs brew update, brew upgrade, and optional
brew cleanup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("no-aur") && app.Platform != platform.Linux {
			return arcerrs.ErrNoAURLinuxOnly
		}
		if cmd.Flags().Changed("yes") && app.Platform != platform.Linux {
			return arcerrs.ErrAssumeYesLinuxOnly
		}
		if cmd.Flags().Changed("log") && app.Platform != platform.Linux {
			return arcerrs.ErrUpdateLogLinuxOnly
		}
		if cmd.Flags().Changed("diff") && app.Platform != platform.Linux {
			return arcerrs.ErrAURDiffLinuxOnly
		}
		return app.PkgMgr.UpdateSystem(pkgmgr.UpdateOptions{
			SkipAUR:   updateNoAUR,
			SkipCache: updateNoCache,
			AssumeYes: updateYes,
			Log:       updateLog,
			Verbose:   updateVerbose,
			ShowDiff:  updateDiff,
		})
	},
}

var updateUvCmd = &cobra.Command{
	Use:   "uv",
	Short: "Update uv package manager",
	Long:  `Update uv to the latest version using uv self update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !run.CommandExists("uv") {
			return shell.NewErrToolNotAvailable("uv")
		}

		output.Title("arc update uv", "")
		output.Section("uv self update")
		if err := run.RunInteractive("uv", "self", "update"); err != nil {
			return fmt.Errorf("failed to update uv: %w", err)
		}

		output.Summary(output.GlyphOK, "uv updated")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.AddCommand(updateSelfCmd, updateSystemCmd, updateUvCmd)
	updateSystemCmd.Flags().BoolVar(&updateNoAUR, "no-aur", false, "Skip AUR updates on Linux")
	updateSystemCmd.Flags().BoolVar(&updateNoCache, "no-cache", false, "Skip cache cleanup")
	updateSystemCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Approve the displayed Linux repository transaction without prompting")
	updateSystemCmd.Flags().BoolVar(&updateLog, "log", false, "Save complete Linux subprocess output to a private update log")
	updateSystemCmd.Flags().BoolVarP(&updateVerbose, "verbose", "v", false, "Print routine warnings that are otherwise only counted in the summary")
	updateSystemCmd.Flags().BoolVar(&updateDiff, "diff", false, "Show every AUR build-file diff, including routine version bumps (Linux)")
}
