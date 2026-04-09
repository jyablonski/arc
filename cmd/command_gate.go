package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const extraCommandsEnvVar = "ARC_EXTRA_COMMANDS"

func normalizeCommandPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, rootCmd.Use+" ")
	return strings.Join(strings.Fields(path), " ")
}

func extraCommandsEnabled() bool {
	value, ok := os.LookupEnv(extraCommandsEnvVar)
	if !ok {
		return false
	}

	enabled, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && enabled
}

func ensureCommandEnabled(cmd *cobra.Command) error {
	if extraCommandsEnabled() {
		return nil
	}

	return fmt.Errorf("command %q is not available", normalizeCommandPath(cmd.CommandPath()))
}

func configureAdminCommands() {
	hidden := !extraCommandsEnabled()
	ghCmd.Hidden = hidden
	ghRestartDashboardCmd.Hidden = hidden
}
