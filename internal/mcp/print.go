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

// PrintSyncHuman renders one sync run per provider onto the stream the
// command titled and the manager reported its steps to, and closes it with a
// verdict line. A dry run reports what it would have changed.
func PrintSyncHuman(st *output.Stream, res SyncResult, dryRun bool) {
	style := st.Style()
	st.Gap()
	grid := output.Grid{Columns: []output.Column{
		{Header: "provider"},
		{Header: "written", Align: output.AlignRight},
		{Header: "removed", Align: output.AlignRight},
		{Header: "conflicts", Align: output.AlignRight},
		{Header: "unsupported", Align: output.AlignRight},
		{Header: "path", Flex: true},
	}}
	if dryRun {
		grid.Columns[1].Header, grid.Columns[2].Header = "to write", "to remove"
	}
	written, removed := 0, 0
	for _, p := range res.Providers {
		written += p.Written
		removed += p.Removed
		grid.Rows = append(grid.Rows, []string{
			p.Provider,
			fmt.Sprintf("%d", p.Written),
			fmt.Sprintf("%d", p.Removed),
			fmt.Sprintf("%d", len(p.Conflicts)),
			fmt.Sprintf("%d", len(p.Unsupported)),
			output.TildePath(p.Path),
		})
	}
	for _, line := range grid.Lines(style, style.Width) {
		st.Line(line)
	}

	// Problems go under the grid as whole lines: a provider's error must not
	// be truncated to fit a column.
	type note struct {
		glyph output.Glyph
		text  string
	}
	var notes []note
	for _, p := range res.Providers {
		if p.Error != "" {
			notes = append(notes, note{output.GlyphFail, fmt.Sprintf("%s: %s", p.Provider, p.Error)})
		}
		for _, name := range p.Conflicts {
			notes = append(notes, note{output.GlyphWarn, fmt.Sprintf("%s/%s: configured by hand and differs; left unchanged", p.Provider, name)})
		}
		for _, name := range sortedKeys(p.Unsupported) {
			notes = append(notes, note{output.GlyphWarn, fmt.Sprintf("%s/%s: skipped, %s", p.Provider, name, p.Unsupported[name])})
		}
	}
	if len(notes) > 0 {
		st.Gap()
		for _, n := range notes {
			st.Step(n.glyph, n.text)
		}
	}

	wrote, gone := "written", "removed"
	if dryRun {
		wrote, gone = "to write", "to remove"
	}
	var changed []string
	if written > 0 {
		changed = append(changed, fmt.Sprintf("%d %s", written, wrote))
	}
	if removed > 0 {
		changed = append(changed, fmt.Sprintf("%d %s", removed, gone))
	}
	switch {
	case res.Failures() > 0:
		st.Summary(output.GlyphFail, append([]string{output.Count(res.Failures(), "provider", "providers") + " failed"}, changed...)...)
	case res.Conflicts() > 0:
		parts := append([]string{output.Count(res.Conflicts(), "conflict", "conflicts")}, changed...)
		st.Summary(output.GlyphWarn, append(parts, style.Faint("arc mcp sync --force"))...)
	case len(changed) == 0:
		st.Summary(output.GlyphOK, "already in sync")
	default:
		st.Summary(output.GlyphOK, changed...)
	}
}

// PrintImportHuman closes an import run: the manager has already announced
// each entry it took, so this adds the ones it would not take and the verdict.
func PrintImportHuman(st *output.Stream, res ImportResult, dryRun bool) {
	canonical := output.TildePath(res.CanonicalFile)
	if len(res.Added) == 0 && len(res.Conflicts) == 0 && len(res.Rejected) == 0 {
		st.Step(output.GlyphInfo, "nothing new to import into "+canonical)
		return
	}
	for _, s := range res.Conflicts {
		st.Step(output.GlyphWarn, fmt.Sprintf("%s (%s): %s", s.Name, s.Provider, s.Reason))
	}
	for _, s := range res.Rejected {
		st.Step(output.GlyphFail, fmt.Sprintf("%s (%s): %s", s.Name, s.Provider, s.Reason))
	}

	glyph := output.GlyphOK
	switch {
	case len(res.Rejected) > 0:
		glyph = output.GlyphFail
	case len(res.Conflicts) > 0:
		glyph = output.GlyphWarn
	}
	imported := fmt.Sprintf("%d imported into %s", len(res.Added), canonical)
	if dryRun {
		imported = fmt.Sprintf("%d would be imported into %s", len(res.Added), canonical)
	}
	parts := []string{imported}
	if n := len(res.Conflicts); n > 0 {
		parts = append(parts, output.Count(n, "conflict", "conflicts"))
	}
	if n := len(res.Rejected); n > 0 {
		parts = append(parts, fmt.Sprintf("%d rejected", n))
	}
	if len(res.Added) > 0 && !dryRun {
		parts = append(parts, st.Style().Faint("arc mcp sync"))
	}
	st.Summary(glyph, parts...)
}
