package sysupdate

import (
	"bufio"
	"fmt"
	"io"
)

type Options struct {
	SkipAUR   bool
	SkipCache bool
	AssumeYes bool
	Log       bool
	// Verbose prints routine warnings that are otherwise only counted.
	Verbose bool
	// ShowDiff expands every AUR build-file diff, not just non-routine ones.
	ShowDiff bool
}

// Run updates the system using [DefaultDeps].
func Run(opts Options) error {
	return RunWithDeps(Deps{}, opts)
}

// promptReboot asks whether to reboot after a kernel update. It renders
// through the run's renderer so the question keeps arc's voice and lands on
// the same stream as everything above it.
func promptReboot(renderer Renderer, stdin io.Reader, runInteractive func(name string, args ...string) error) error {
	renderer.Blank()
	renderer.Warning("a kernel update was installed; a reboot is required for it to take effect")
	renderer.Prompt("Reboot now?", true)

	reader := bufio.NewReader(stdin)
	response, err := reader.ReadString('\n')
	renderer.EndPrompt()
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if parseRebootConfirmation(response) {
		renderer.Info("rebooting now")
		if err := runInteractive("sudo", "reboot"); err != nil {
			return fmt.Errorf("failed to reboot: %w", err)
		}
	} else {
		renderer.Info("reboot skipped; reboot manually when convenient")
	}

	return nil
}
