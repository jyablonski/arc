package cmd

import (
	"fmt"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var (
	updateNoAUR   bool
	updateNoCache bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Run system updates (pacman, yay, cache cleanup)",
	Long: `Update the system by running pacman -Syu, optionally yay -Syu --aur,
and cleaning the package cache with paccache -rv.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now()
		output.Header(fmt.Sprintf("hello jacob, running updates for %s", now.Format("2006-01-02 15:04:05")))

		// Check if pacman is available
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		// Run pacman update
		output.Info("Running pacman -Syu...")
		if err := shell.RunInteractive("sudo", "pacman", "-Syu"); err != nil {
			return fmt.Errorf("pacman update failed: %w", err)
		}

		// Run yay update if not disabled
		if !updateNoAUR {
			if pacman.CheckYayAvailable() {
				output.Info("Running yay -Syu --aur...")
				if err := shell.RunInteractive("yay", "-Syu", "--aur"); err != nil {
					output.Warning(fmt.Sprintf("yay update failed: %v", err))
				}
			} else {
				output.Warning("yay is not available, skipping AUR updates")
			}
		}

		// Clean package cache if not disabled
		if !updateNoCache {
			output.Info("Cleaning package cache...")
			if _, err := shell.RunSudo("paccache", "-rv"); err != nil {
				output.Warning(fmt.Sprintf("paccache failed: %v", err))
			}
		}

		output.Header(fmt.Sprintf("updates complete at %s, hasta la vista", time.Now().Format("2006-01-02 15:04:05")))
		return nil
	},
}

var updateUvCmd = &cobra.Command{
	Use:   "uv",
	Short: "Update uv package manager",
	Long:  `Update uv to the latest version using uv self update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !shell.CommandExists("uv") {
			return fmt.Errorf("uv is not available in PATH")
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
	updateCmd.Flags().BoolVar(&updateNoAUR, "no-aur", false, "Skip AUR updates")
	updateCmd.Flags().BoolVar(&updateNoCache, "no-cache", false, "Skip cache cleanup")
	updateCmd.AddCommand(updateUvCmd)
}
