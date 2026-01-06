package cmd

import (
	"fmt"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/pacman"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [package]",
	Short: "Search for packages",
	Long:  `Search for packages using pacman -Ss.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pacman.CheckPacmanAvailable(); err != nil {
			return err
		}

		query := args[0]
		results, err := pacman.SearchPackages(query)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(results) == 0 {
			output.Info("No packages found")
			return nil
		}

		for _, result := range results {
			fmt.Println(result)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
