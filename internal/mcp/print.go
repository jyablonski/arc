package mcp

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyablonski/arc/internal/output"
)

// PrintListHuman renders canonical MCP configuration entries as one row per
// entry with a glyph column per provider, the same shape as "arc skills list".
// The set column says whether the env vars an entry needs exist in this shell,
// the most likely reason a server that lists fine fails at runtime.
func PrintListHuman(w io.Writer, providers []Provider, res ListResult) {
	style := output.StyleFor(w)
	meta := output.Count(len(res.Servers), "server", "servers") + style.Sep() + output.TildePath(res.CanonicalFile)
	sc := &output.Screen{W: w, Style: style, Title: "arc mcp list", Meta: meta}
	if len(res.Servers) == 0 {
		sc.Notes(unmanagedNotes(style, res))
		sc.Flush(style.Glyph(output.GlyphInfo) + " no MCP configuration in " + output.TildePath(res.CanonicalFile) +
			style.Sep() + style.Faint("arc mcp import"))
		return
	}

	grid := output.Grid{Columns: []output.Column{
		{Header: "name"},
		{Header: "type"},
		{Header: "env"},
		{Header: "set", Align: output.AlignCenter},
	}}
	for _, p := range providers {
		grid.Columns = append(grid.Columns, output.Column{Header: p.Name(), Align: output.AlignCenter})
	}
	unset := map[string]string{}
	for _, s := range res.Servers {
		name := s.Name
		if !s.Enabled {
			name += style.Faint(" (off)")
		}
		env, set := style.Dash(), style.Dash()
		if len(s.EnvRefs) > 0 {
			env = strings.Join(s.EnvRefs, ",")
			set = style.Glyph(output.GlyphOK)
			for _, ref := range s.EnvRefs {
				if os.Getenv(ref) == "" {
					unset[ref] = ref
					set = style.Glyph(output.GlyphWarn)
				}
			}
		}
		row := []string{name, string(s.Type), env, set}
		for _, p := range providers {
			row = append(row, style.Glyph(statusGlyph(s.Providers[p.Name()].Status)))
		}
		grid.Rows = append(grid.Rows, row)
	}
	sc.Grid(grid)

	var notes []output.Note
	for _, ref := range sortedKeys(unset) {
		notes = append(notes, output.Note{Glyph: output.GlyphWarn, Label: "env not set", Detail: ref, Hint: "export it before launching the tool"})
	}
	notes = append(notes, detailNotes(style, providers, res)...)
	notes = append(notes, unmanagedNotes(style, res)...)
	if len(notes) > 0 {
		sc.Blank()
		sc.Notes(notes)
	}
	sc.Flush(CheckLine(style, len(providers), res))
}

// detailNotes explains every non-ok cell, since "unsupported" and "conflict"
// are useless without the reason.
func detailNotes(style output.Style, providers []Provider, res ListResult) []output.Note {
	var notes []output.Note
	for _, s := range res.Servers {
		for _, p := range providers {
			ps := s.Providers[p.Name()]
			if ps.Status == StatusOK || ps.Status == StatusDisabled || ps.Status == StatusExcluded {
				continue
			}
			detail := fmt.Sprintf("%s %s %s", p.Name(), style.Sym("›", ">"), s.Name)
			if ps.Detail != "" {
				detail += "  " + ps.Detail
			}
			notes = append(notes, output.Note{Glyph: statusGlyph(ps.Status), Label: string(ps.Status), Detail: detail, Hint: statusHint[ps.Status]})
		}
	}
	return notes
}

func unmanagedNotes(style output.Style, res ListResult) []output.Note {
	var notes []output.Note
	label := fmt.Sprintf("%d unmanaged", len(res.Unmanaged))
	for _, u := range res.Unmanaged {
		notes = append(notes, output.Note{Glyph: output.GlyphWarn, Label: label,
			Detail: fmt.Sprintf("%s %s %s", u.Provider, style.Sym("›", ">"), u.Name), Hint: "arc mcp import"})
		label = ""
	}
	return notes
}

// OutOfSync counts provider cells that sync would change or that need a human.
func (r ListResult) OutOfSync() int {
	n := 0
	for _, s := range r.Servers {
		for _, ps := range s.Providers {
			switch ps.Status {
			case StatusMissing, StatusDrift, StatusConflict:
				n++
			}
		}
	}
	return n
}

// CheckLine is the one-line verdict shared by the list footer and --check.
func CheckLine(style output.Style, providers int, res ListResult) string {
	sep := style.Sep()
	if len(res.Servers) == 0 {
		return style.Glyph(output.GlyphInfo) + " no MCP configuration" + sep + style.Faint("arc mcp import")
	}
	if n := res.OutOfSync(); n > 0 {
		return style.Glyph(output.GlyphWarn) + " " + output.Count(n, "entry", "entries") + " out of sync" + sep + style.Faint("arc mcp sync")
	}
	line := style.Glyph(output.GlyphOK) + " " + fmt.Sprintf("%s in sync across %s",
		output.Count(len(res.Servers), "server", "servers"), output.Count(providers, "provider", "providers"))
	if n := len(res.Unmanaged); n > 0 {
		line += sep + fmt.Sprintf("%d unmanaged", n)
	}
	return line
}

func statusGlyph(st Status) output.Glyph {
	switch st {
	case StatusOK:
		return output.GlyphOK
	case StatusMissing, StatusDisabled, StatusExcluded:
		return output.GlyphInfo
	case StatusDrift:
		return output.GlyphDrift
	case StatusUnsupported:
		return output.GlyphFail
	default:
		return output.GlyphWarn
	}
}

var statusHint = map[Status]string{
	StatusMissing:     "arc mcp sync",
	StatusDrift:       "arc mcp sync",
	StatusConflict:    "arc mcp sync --force",
	StatusUnsupported: "restrict it with --restrict-to",
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
