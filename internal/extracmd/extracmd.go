package extracmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const EnvVar = "ARC_EXTRA_COMMANDS"

var admin []*cobra.Command

func RegisterHiddenUnlessEnabled(cmds ...*cobra.Command) {
	admin = append(admin, cmds...)
}

func Enabled() bool {
	value, ok := os.LookupEnv(EnvVar)
	if !ok {
		return false
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && enabled
}

func ApplyVisibility() {
	hidden := !Enabled()
	for _, c := range admin {
		if c != nil {
			c.Hidden = hidden
		}
	}
}

func NormalizeRelativePath(cmd *cobra.Command) string {
	path := strings.TrimSpace(cmd.CommandPath())
	root := cmd.Root().Name()
	prefix := root + " "
	if strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
	}
	return strings.Join(strings.Fields(path), " ")
}

func EnsureAvailable(cmd *cobra.Command) error {
	if Enabled() {
		return nil
	}
	return fmt.Errorf("command %q is not available", NormalizeRelativePath(cmd))
}
