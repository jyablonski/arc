package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/pacman"
	"github.com/spf13/cobra"
)

var (
	installedAUROnly bool
	installedCount   bool
)

var installedCmd = &cobra.Command{
	Use:   "installed",
	Short: "List explicitly installed packages",
	Long:  `List packages that were explicitly installed (not as dependencies).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		var packages []string
		var err error

		if installedAUROnly {
			packages, err = pacman.GetForeignPackages()
			if err != nil {
				return fmt.Errorf("failed to get foreign packages: %w", err)
			}
		} else {
			packages, err = pacman.GetExplicitlyInstalled()
			if err != nil {
				return fmt.Errorf("failed to get installed packages: %w", err)
			}
		}

		if installedCount {
			fmt.Println(len(packages))
		} else {
			for _, pkg := range packages {
				fmt.Println(pkg)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installedCmd)
	installedCmd.Flags().BoolVar(&installedAUROnly, "aur-only", false, "Show only AUR packages")
	installedCmd.Flags().BoolVar(&installedCount, "count", false, "Show only the count")
}
