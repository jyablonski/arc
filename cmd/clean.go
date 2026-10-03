package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/sysupdate"
	"github.com/spf13/cobra"
)

var (
	cleanOrphansOnly bool
	cleanCacheOnly   bool
	cleanLogsOnly    bool
	cleanUpdateLogs  = sysupdate.CleanUpdateLogs
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean package cache, unused dependencies, and Arc update logs",
	Long: `Clean package-manager cache, unused dependencies, and Arc update logs.
Linux uses pacman -Sc and pacman -Rns. macOS uses brew cleanup and brew autoremove.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		selected := 0
		for _, enabled := range []bool{cleanOrphansOnly, cleanCacheOnly, cleanLogsOnly} {
			if enabled {
				selected++
			}
		}
		if selected > 1 {
			return fmt.Errorf("only one of --orphans-only, --cache-only, or --logs-only may be used")
		}

		output.Title("arc clean", app.Platform.String())
		if !cleanLogsOnly {
			if err := app.PkgMgr.Clean(pkgmgr.CleanOptions{
				OrphansOnly: cleanOrphansOnly,
				CacheOnly:   cleanCacheOnly,
			}); err != nil {
				return err
			}
		}
		if selected == 0 || cleanLogsOnly {
			if err := runLogCleanup(); err != nil {
				return err
			}
		}
		output.Summary(output.GlyphOK, "clean complete")
		return nil
	},
}

func runLogCleanup() error {
	result, err := cleanUpdateLogs()
	if err != nil {
		return fmt.Errorf("failed to clean Arc update logs: %w", err)
	}
	if result.Files == 0 {
		output.Info("no update logs to remove")
		return nil
	}
	output.Success(logCleanupMessage(result))
	return nil
}

func logCleanupMessage(result sysupdate.LogCleanupResult) string {
	return fmt.Sprintf("removed %s (%s)", output.Count(result.Files, "update log", "update logs"), output.Bytes(result.Bytes))
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanOrphansOnly, "orphans-only", false, "Only remove orphaned packages")
	cleanCmd.Flags().BoolVar(&cleanCacheOnly, "cache-only", false, "Only clean package cache")
	cleanCmd.Flags().BoolVar(&cleanLogsOnly, "logs-only", false, "Only remove Arc update logs")
	cleanCmd.MarkFlagsMutuallyExclusive("orphans-only", "cache-only", "logs-only")
}
