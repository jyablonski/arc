package cmd

import (
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/selfupdate"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/jyablonski/arc/internal/sysupdate"
	"github.com/spf13/cobra"
)

var (
	updateNoAUR   bool
	updateNoCache bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Run updates for arc, the system, or tools",
	Long: `Subcommands pick what to upgrade:

  self   — Fetch the latest arc release from GitHub and replace this binary.

  system — Full Arch workflow: pacman keyring, pacman -Syu, optional yay (--aur),
           optional paccache cleanup, kernel bump detection and reboot prompt.

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
	Short: "Run system updates (pacman, yay, cache cleanup)",
	Long: `Update the system by running pacman -Syu, optionally yay -Syu --aur,
and cleaning the package cache with paccache -rv.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return sysupdate.Run(sysupdate.Options{
			SkipAUR:   updateNoAUR,
			SkipCache: updateNoCache,
		})
	},
}

var updateUvCmd = &cobra.Command{
	Use:   "uv",
	Short: "Update uv package manager",
	Long:  `Update uv to the latest version using uv self update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("uv") {
			return shell.NewErrToolNotAvailable("uv")
		}

		output.Info("Updating uv...")
		if err := shell.RunInteractive("uv", "self", "update"); err != nil {
			return fmt.Errorf("failed to update uv: %w", err)
		}

		output.Success("uv updated successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.AddCommand(updateSelfCmd, updateSystemCmd, updateUvCmd)
	updateSystemCmd.Flags().BoolVar(&updateNoAUR, "no-aur", false, "Skip AUR updates")
	updateSystemCmd.Flags().BoolVar(&updateNoCache, "no-cache", false, "Skip cache cleanup")
}
