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

		style := output.StyleFor(os.Stdout)
		sc := &output.Screen{W: os.Stdout, Style: style, Title: "arc stats", Meta: output.Timestamp(time.Now())}
		if report.Total == 0 {
			sc.Flush(style.Glyph(output.GlyphInfo) + " no invocations recorded yet")
			return nil
		}

		grid := output.Grid{Columns: []output.Column{
			{Header: "command"},
			{Header: "count", Align: output.AlignRight},
			{Header: "failures", Align: output.AlignRight},
			{Header: "last used"},
			{Header: "total time", Align: output.AlignRight},
		}}
		failures := 0
		for _, cs := range report.Commands {
			failures += cs.Failures
			grid.Rows = append(grid.Rows, []string{
				cs.Command,
				fmt.Sprintf("%d", cs.Count),
				fmt.Sprintf("%d", cs.Failures),
				output.Timestamp(cs.LastUsed.Local()),
				output.Duration(time.Duration(cs.TotalMS) * time.Millisecond),
			})
		}
		sc.Grid(grid)
		sc.Flush(style.Faint(strings.Join([]string{
			output.Count(len(report.Commands), "command", "commands"),
			output.Count(report.Total, "invocation", "invocations"),
			output.Count(failures, "failure", "failures"),
		}, style.Sep())))
		return nil
	},
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
