package mcp

import (
	"fmt"
	"io"
	"strings"

	"github.com/jyablonski/arc/internal/output"
)

// PrintListHuman renders the canonical servers as one row per server with a
// column per provider, the same shape as "arc skills list".
func PrintListHuman(w io.Writer, providers []Provider, res ListResult) {
	if len(res.Servers) == 0 {
		_, _ = fmt.Fprintf(w, "no MCP servers in %s\n", res.CanonicalFile)
		_, _ = fmt.Fprintln(w, "run 'arc mcp import' to seed it from your existing tool configs")
		printUnmanaged(w, res)
		return
	}

	headers := []string{"name", "type", "env"}
	for _, p := range providers {
		headers = append(headers, p.Name())
	}
	rows := make([][]string, 0, len(res.Servers))
	for _, s := range res.Servers {
		name := s.Name
		if !s.Enabled {
			name += " (off)"
		}
		row := []string{name, string(s.Type), strings.Join(s.EnvRefs, ",")}
		for _, p := range providers {
			row = append(row, string(s.Providers[p.Name()].Status))
		}
		rows = append(rows, row)
	}
	output.FprintTable(w, headers, rows)

	printDetails(w, providers, res)
	printUnmanaged(w, res)
}

// printDetails explains every non-ok cell, since "unsupported" and "conflict"
// are useless without the reason.
func printDetails(w io.Writer, providers []Provider, res ListResult) {
	var lines []string
	for _, s := range res.Servers {
		for _, p := range providers {
			ps := s.Providers[p.Name()]
			if ps.Detail == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %s/%s: %s (%s)", p.Name(), s.Name, ps.Detail, ps.Status))
		}
	}
	if len(lines) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	for _, l := range lines {
		_, _ = fmt.Fprintln(w, l)
	}
}

func printUnmanaged(w io.Writer, res ListResult) {
	if len(res.Unmanaged) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Configured in a provider but not canonical (arc leaves these alone):")
	for _, u := range res.Unmanaged {
		_, _ = fmt.Fprintf(w, "  %s: %s\n", u.Provider, u.Name)
	}
	_, _ = fmt.Fprintln(w, "  run 'arc mcp import' to adopt them")
}

// PrintSyncHuman summarizes one sync run per provider.
func PrintSyncHuman(w io.Writer, res SyncResult) {
	headers := []string{"provider", "written", "removed", "conflicts", "unsupported", "path"}
	rows := make([][]string, 0, len(res.Providers))
	for _, p := range res.Providers {
		status := p.Path
		if p.Error != "" {
			status = "error: " + p.Error
		}
		rows = append(rows, []string{
			p.Provider,
			fmt.Sprintf("%d", p.Written),
			fmt.Sprintf("%d", p.Removed),
			fmt.Sprintf("%d", len(p.Conflicts)),
			fmt.Sprintf("%d", len(p.Unsupported)),
			status,
		})
	}
	output.FprintTable(w, headers, rows)

	for _, p := range res.Providers {
		for _, name := range p.Conflicts {
			output.Warning(fmt.Sprintf("%s/%s: configured by hand and differs; left unchanged (use --force to overwrite)", p.Provider, name))
		}
		for _, name := range sortedKeys(p.Unsupported) {
			output.Warning(fmt.Sprintf("%s/%s: skipped, %s", p.Provider, name, p.Unsupported[name]))
		}
	}
}

// PrintImportHuman summarizes what import pulled into canonical.
func PrintImportHuman(w io.Writer, res ImportResult) {
	_, _ = fmt.Fprintf(w, "canonical: %s\n", res.CanonicalFile)
	if len(res.Added) == 0 && len(res.Conflicts) == 0 && len(res.Rejected) == 0 {
		_, _ = fmt.Fprintln(w, "nothing new to import")
		return
	}
	for _, s := range res.Added {
		output.Success(fmt.Sprintf("imported %s from %s", s.Name, s.Provider))
	}
	for _, s := range res.Conflicts {
		output.Warning(fmt.Sprintf("%s (%s): %s", s.Name, s.Provider, s.Reason))
	}
	for _, s := range res.Rejected {
		output.Error(fmt.Sprintf("%s (%s): %s", s.Name, s.Provider, s.Reason))
	}
	if len(res.Added) > 0 {
		output.Info("run 'arc mcp sync' to push them to every provider")
	}
}
