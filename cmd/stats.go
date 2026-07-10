package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jyablonski/arc/internal/output"
	"github.com/jyablonski/arc/internal/stats"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show arc command usage statistics",
	Long: `Show how often each arc command has been run on this machine.

Every invocation appends one record — command path, outcome, and duration —
to a local log file. Arguments and flag values are never stored, and nothing
leaves the machine. Set ` + stats.NoTrackEnvVar + `=1 to disable tracking.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := stats.ReadAll()
		if err != nil {
			return err
		}
		report := stats.Aggregate(entries)

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		if report.Total == 0 {
			output.Info("no invocations recorded yet")
			return nil
		}

		output.Header(fmt.Sprintf("Command Usage (%d invocations)", report.Total))
		rows := make([][]string, 0, len(report.Commands))
		for _, cs := range report.Commands {
			rows = append(rows, []string{
				cs.Command,
				fmt.Sprintf("%d", cs.Count),
				fmt.Sprintf("%d", cs.Failures),
				cs.LastUsed.Local().Format("2006-01-02 15:04"),
				formatTotalDuration(cs.TotalMS),
			})
		}
		output.Table([]string{"COMMAND", "COUNT", "FAILURES", "LAST USED", "TOTAL TIME"}, rows)
		return nil
	},
}

func formatTotalDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d >= time.Second {
		d = d.Round(time.Second)
	}
	return d.String()
}

// recordInvocation appends the executed command to the local stats log. It is
// best-effort by design: any failure is swallowed so tracking can never break
// or fail the command the user actually ran.
func recordInvocation(c *cobra.Command, ok bool, elapsed time.Duration) {
	if c == nil || !stats.Enabled() {
		return
	}
	name := commandRelPath(c)
	if !trackable(name) {
		return
	}
	_ = stats.Append(stats.Entry{
		Timestamp:  time.Now().UTC(),
		Command:    name,
		OK:         ok,
		DurationMS: elapsed.Milliseconds(),
	})
}

// commandRelPath returns the command path without the root name, e.g.
// "update system"; empty for the root command itself.
func commandRelPath(c *cobra.Command) string {
	path := strings.TrimPrefix(c.CommandPath(), c.Root().Name())
	return strings.TrimSpace(path)
}

// trackable excludes the root command (bare `arc` just prints help), shell
// completion machinery, help, and stats itself from tracking.
func trackable(name string) bool {
	if name == "" {
		return false
	}
	switch strings.Fields(name)[0] {
	case "help", "completion", "__complete", "__completeNoDesc", "stats":
		return false
	}
	return true
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
