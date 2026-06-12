package hardware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/platform"
)

var ErrUnsupportedPlatform = errors.New("hardware info is not supported on this platform")

//go:generate go tool moq -rm -out reporter_moq.go . Reporter
type Reporter interface {
	Show(components []string) error
}

func New(os platform.OS) Reporter {
	switch os {
	case platform.Linux:
		return linuxReporter{}
	case platform.Darwin:
		return darwinReporter{}
	default:
		return unsupportedReporter{}
	}
}

type linuxReporter struct{}

func (linuxReporter) Show(components []string) error {
	for _, comp := range components {
		switch comp {
		case "mobo":
			output.Header("motherboard is")
			result, err := run.RunSudo("dmidecode", "-t", "2")
			if err != nil {
				return fmt.Errorf("failed to get motherboard info: %w", err)
			}
			fmt.Println(result)
		case "cpu":
			output.Header("cpu is")
			result, err := run.RunSudo("dmidecode", "-t", "4")
			if err != nil {
				return fmt.Errorf("failed to get CPU info: %w", err)
			}
			fmt.Println(result)
		case "gpu":
			output.Header("gpu is")
			pciOutput, err := run.Run("lspci")
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
						gpuInfo, err := run.Run("lspci", "-v", "-s", parts[0])
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
			result, err := run.Run("nvidia-smi")
			if err != nil {
				return fmt.Errorf("nvidia-smi failed: %w", err)
			}
			fmt.Println(result)
		case "ram":
			output.Header("ram is")
			result, err := run.RunSudo("lshw", "-C", "memory")
			if err != nil {
				return fmt.Errorf("failed to get RAM info: %w", err)
			}
			fmt.Println(result)
		default:
			return fmt.Errorf("unknown component: %s (valid: mobo, cpu, gpu, gpu-driver, ram)", comp)
		}
	}
	return nil
}

type darwinReporter struct{}

func (darwinReporter) Show(components []string) error {
	for _, comp := range components {
		switch comp {
		case "mobo":
			output.Header("hardware is")
			result, err := run.Run("system_profiler", "SPHardwareDataType")
			if err != nil {
				return fmt.Errorf("failed to get hardware info: %w", err)
			}
			fmt.Println(result)
		case "cpu":
			output.Header("cpu is")
			result, err := run.Run("sysctl", "-n", "machdep.cpu.brand_string")
			if err != nil {
				return fmt.Errorf("failed to get CPU info: %w", err)
			}
			fmt.Println(result)
		case "gpu":
			output.Header("gpu is")
			result, err := run.Run("system_profiler", "SPDisplaysDataType")
			if err != nil {
				return fmt.Errorf("failed to get GPU info: %w", err)
			}
			fmt.Println(result)
		case "gpu-driver":
			return fmt.Errorf("gpu-driver is only supported on Linux")
		case "ram":
			output.Header("ram is")
			result, err := run.Run("system_profiler", "SPMemoryDataType")
			if err != nil {
				return fmt.Errorf("failed to get RAM info: %w", err)
			}
			fmt.Println(result)
		default:
			return fmt.Errorf("unknown component: %s (valid: mobo, cpu, gpu, gpu-driver, ram)", comp)
		}
	}
	return nil
}

type unsupportedReporter struct{}

func (unsupportedReporter) Show([]string) error {
	return ErrUnsupportedPlatform
}
