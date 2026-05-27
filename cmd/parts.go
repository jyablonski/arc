package cmd

import (
	"github.com/spf13/cobra"
)

var partsComponent string

var partsCmd = &cobra.Command{
	Use:   "parts",
	Short: "Show hardware information",
	Long: `Display hardware information including motherboard, CPU, GPU, and RAM details.
Use --component to show only a specific component.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		components := []string{"mobo", "cpu", "gpu", "gpu-driver", "ram"}
		if partsComponent != "" {
			components = []string{partsComponent}
		}
		return app.Hardware.Show(components)
	},
}

func init() {
	rootCmd.AddCommand(partsCmd)
	partsCmd.Flags().StringVar(&partsComponent, "component", "", "Show only specific component (mobo, cpu, gpu, gpu-driver, ram)")
}
