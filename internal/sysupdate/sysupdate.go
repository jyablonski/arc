package sysupdate

import (
	"bufio"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/output"
)

type Options struct {
	SkipAUR   bool
	SkipCache bool
}

// Run updates the system using [DefaultDeps].
func Run(opts Options) error {
	return RunWithDeps(Deps{}, opts)
}

// promptReboot asks whether to reboot after a kernel update.
func promptReboot(stdin *os.File, runInteractive func(name string, args ...string) error) error {
	output.Warning("A kernel update was successfully installed. A reboot is required for the changes to take effect.")
	fmt.Print("Reboot now? [Y/n]: ")

	reader := bufio.NewReader(stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if parseRebootConfirmation(response) {
		output.Info("Rebooting now...")
		if err := runInteractive("sudo", "reboot"); err != nil {
			return fmt.Errorf("failed to reboot: %w", err)
		}
	} else {
		output.Info("Reboot skipped. Please reboot manually when convenient.")
	}

	return nil
}
