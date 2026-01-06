package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/jyablonski/arc/internal/shell"
	"github.com/spf13/cobra"
)

var (
	cleanOrphansOnly bool
	cleanCacheOnly   bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean package cache and remove orphaned packages",
	Long: `Clean the package cache with pacman -Sc and remove orphaned packages
with pacman -Rns.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		if !cleanOrphansOnly {
			output.Header("cleaning package cache")
			if _, err := shell.RunSudo("pacman", "-Sc", "--noconfirm"); err != nil {
				return fmt.Errorf("failed to clean cache: %w", err)
			}
			output.Success("Package cache cleaned")
		}

		if !cleanCacheOnly {
			output.Header("removing orphaned packages")
			orphans, err := pacman.GetOrphanedPackages()
			if err != nil {
				return fmt.Errorf("failed to get orphaned packages: %w", err)
			}

			if len(orphans) == 0 {
				output.Info("No orphans to remove")
			} else {
				args := append([]string{"pacman", "-Rns", "--noconfirm"}, orphans...)
				if _, err := shell.RunSudo(args[0], args[1:]...); err != nil {
					output.Warning(fmt.Sprintf("Failed to remove some orphans: %v", err))
				} else {
					output.Success(fmt.Sprintf("Removed %d orphaned packages", len(orphans)))
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanOrphansOnly, "orphans-only", false, "Only remove orphaned packages")
	cleanCmd.Flags().BoolVar(&cleanCacheOnly, "cache-only", false, "Only clean package cache")
}
