package cmd

import (
	"fmt"
	"strings"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/shell"
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

		for _, comp := range components {
			switch comp {
			case "mobo":
				output.Header("motherboard is")
				result, err := shell.RunSudo("dmidecode", "-t", "2")
				if err != nil {
					return fmt.Errorf("failed to get motherboard info: %w", err)
				}
				fmt.Println(result)

			case "cpu":
				output.Header("cpu is")
				result, err := shell.RunSudo("dmidecode", "-t", "4")
				if err != nil {
					return fmt.Errorf("failed to get CPU info: %w", err)
				}
				fmt.Println(result)

			case "gpu":
				output.Header("gpu is")
				// Get GPU PCI address
				pciOutput, err := shell.Run("lspci")
				if err != nil {
					return fmt.Errorf("failed to get GPU PCI info: %w", err)
				}
				lines := strings.Split(pciOutput, "\n")
				found := false
				for _, line := range lines {
					if strings.Contains(line, " VGA ") {
						found = true
						parts := strings.Fields(line)
						if len(parts) > 0 {
							pciAddr := parts[0]
							gpuInfo, err := shell.Run("lspci", "-v", "-s", pciAddr)
							if err == nil {
								fmt.Println(gpuInfo)
							}
						}
					}
				}
				if !found {
					output.Warning("no VGA-compatible GPU found via lspci")
				}

			case "gpu-driver":
				output.Header("gpu driver is")
				result, err := shell.Run("nvidia-smi")
				if err != nil {
					return fmt.Errorf("nvidia-smi failed: %w", err)
				}
				fmt.Println(result)

			case "ram":
				output.Header("ram is")
				result, err := shell.RunSudo("lshw", "-C", "memory")
				if err != nil {
					return fmt.Errorf("failed to get RAM info: %w", err)
				}
				fmt.Println(result)

			default:
				return fmt.Errorf("unknown component: %s (valid: mobo, cpu, gpu, gpu-driver, ram)", comp)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(partsCmd)
	partsCmd.Flags().StringVar(&partsComponent, "component", "", "Show only specific component (mobo, cpu, gpu, gpu-driver, ram)")
}
