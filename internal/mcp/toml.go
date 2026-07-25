package mcp

import (
	"slices"
	"strconv"
	"strings"
)

// A deliberately small TOML layer: arc only ever reads and rewrites
// [mcp_servers.*] tables, whose values are strings, string arrays, string
// tables, and bools. Everything else in ~/.codex/config.toml — the model
// setting, sixteen [projects.*] trust entries, comments, blank lines, ordering
// — is carried through as untouched source lines rather than parsed and
// re-marshalled, which is what a general TOML round-trip would cost.

// tomlSection is one [table] block, tracked as a line range.
type tomlSection struct {
	path  []string
	start int // index of the header line
	end   int // exclusive
	array bool
}

type tomlDoc struct {
	lines    []string
	sections []tomlSection
}

func parseTOMLDoc(data []byte) *tomlDoc {
	text := string(data)
	doc := &tomlDoc{}
	if text == "" {
		return doc
	}
	doc.lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	depth := 0
	for i, ln := range doc.lines {
		// Only look for a header at value depth zero: a line inside a
		// multi-line array can look like a table header.
		if depth == 0 {
			if path, ok := parseTableHeader(ln); ok {
				if n := len(doc.sections); n > 0 {
					doc.sections[n-1].end = i
				}
				doc.sections = append(doc.sections, tomlSection{path: path, start: i, end: len(doc.lines)})
				continue
			}
			if path, ok := parseArrayTableHeader(ln); ok {
				if n := len(doc.sections); n > 0 {
					doc.sections[n-1].end = i
				}
				doc.sections = append(doc.sections, tomlSection{
					path: path, start: i, end: len(doc.lines), array: true,
				})
				continue
			}
		}
		depth += bracketDelta(ln)
		if depth < 0 {
			depth = 0
		}
	}
	return doc
}

// parseArrayTableHeader recognizes [[a.b]] so array-of-table declarations can
// terminate the previous section. Arc does not interpret their contents.
func parseArrayTableHeader(line string) ([]string, bool) {
	t := strings.TrimSpace(stripTOMLComment(line))
	if len(t) < 5 || !strings.HasPrefix(t, "[[") || !strings.HasSuffix(t, "]]") {
		return nil, false
	}
	inner := strings.TrimSpace(t[2 : len(t)-2])
	path := splitTOMLKeyPath(inner)
	if len(path) == 0 {
		return nil, false
	}
	return path, true
}

func (d *tomlDoc) render() string {
	if len(d.lines) == 0 {
		return ""
	}
	return strings.Join(d.lines, "\n") + "\n"
}

// body returns a section's lines excluding its header.
func (d *tomlDoc) body(s tomlSection) []string {
	if s.start+1 >= s.end {
		return nil
	}
	return d.lines[s.start+1 : s.end]
}

// parseTableHeader recognizes `[a.b."c"]`. Array-of-tables headers (`[[x]]`)
// are not table headers and are left to the untouched-source path.
func parseTableHeader(line string) ([]string, bool) {
	t := strings.TrimSpace(stripTOMLComment(line))
	if len(t) < 3 || t[0] != '[' || t[len(t)-1] != ']' {
		return nil, false
	}
	if strings.HasPrefix(t, "[[") {
		return nil, false
	}
	inner := strings.TrimSpace(t[1 : len(t)-1])
	if inner == "" {
		return nil, false
	}
	path := splitTOMLKeyPath(inner)
	if len(path) == 0 {
		return nil, false
	}
	return path, true
}

// splitTOMLKeyPath splits a dotted key path, honoring quoted segments.
func splitTOMLKeyPath(s string) []string {
	var out []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			quote = c
		case c == '.':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, strings.TrimSpace(cur.String()))
	if slices.Contains(out, "") {
		return nil
	}
	return out
}

// stripTOMLComment removes a trailing `#` comment that is not inside a string.
func stripTOMLComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return line[:i]
		}
	}
	return line
}

// bracketDelta reports how far a line opens or closes bracketed values,
// ignoring brackets inside strings and comments.
func bracketDelta(line string) int {
	s := stripTOMLComment(line)
	delta := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '{':
			delta++
		case c == ']' || c == '}':
			delta--
		}
	}
	return delta
}

// parseTOMLKeyValues collects `key = value` pairs from a table body, keeping
// values as raw text and joining ones that span multiple lines.
func parseTOMLKeyValues(lines []string) map[string]string {
	out := map[string]string{}
	var key string
	var buf strings.Builder
	depth := 0

	flush := func() {
		out[key] = strings.TrimSpace(buf.String())
		key = ""
		buf.Reset()
		depth = 0
	}

	for _, ln := range lines {
		seg := stripTOMLComment(ln)
		if key == "" {
			trimmed := strings.TrimSpace(seg)
			if trimmed == "" {
				continue
			}
			k, v, ok := splitTOMLKeyValue(trimmed)
			if !ok {
				continue
			}
			key = k
			buf.WriteString(v)
			depth = bracketDelta(v)
			if depth <= 0 {
				flush()
			}
			continue
		}
		buf.WriteString(" ")
		buf.WriteString(strings.TrimSpace(seg))
		depth += bracketDelta(seg)
		if depth <= 0 {
			flush()
		}
	}
	if key != "" {
		flush()
	}
	return out
}

// splitTOMLKeyValue splits on the first `=` outside a quoted key.
func splitTOMLKeyValue(line string) (key, value string, ok bool) {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '=':
			k := strings.TrimSpace(line[:i])
			if k == "" {
				return "", "", false
			}
			if len(k) >= 2 && (k[0] == '"' || k[0] == '\'') {
				k = k[1 : len(k)-1]
			}
			return k, strings.TrimSpace(line[i+1:]), true
		}
	}
	return "", "", false
}

// tomlUnquote turns a TOML string literal into its value. Non-strings come back
// unchanged with ok=false so callers can tell the difference.
func tomlUnquote(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 2 {
		return s, false
	}
	switch s[0] {
	case '"':
		if v, err := strconv.Unquote(s); err == nil {
			return v, true
		}
		if s[len(s)-1] == '"' {
			return s[1 : len(s)-1], true
		}
	case '\'':
		if s[len(s)-1] == '\'' {
			return s[1 : len(s)-1], true
		}
	}
	return s, false
}

func tomlStringArray(raw string) ([]string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, true
	}
	var out []string
	for _, part := range splitTopLevel(inner) {
		v, ok := tomlUnquote(part)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

func tomlStringTable(raw string) (map[string]string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	out := map[string]string{}
	if inner == "" {
		return out, true
	}
	for _, part := range splitTopLevel(inner) {
		k, v, ok := splitTOMLKeyValue(strings.TrimSpace(part))
		if !ok {
			return nil, false
		}
		val, ok := tomlUnquote(v)
		if !ok {
			return nil, false
		}
		out[k] = val
	}
	return out, true
}

func tomlBool(raw string) (bool, bool) {
	switch strings.TrimSpace(raw) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// splitTopLevel splits on commas that are not inside strings or nested
// brackets.
func splitTopLevel(s string) []string {
	var out []string
	var cur strings.Builder
	var quote byte
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == '\\' && quote == '"' && i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
				continue
			}
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == '[' || c == '{':
			depth++
			cur.WriteByte(c)
		case c == ']' || c == '}':
			depth--
			cur.WriteByte(c)
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if last := strings.TrimSpace(cur.String()); last != "" {
		out = append(out, last)
	}
	return out
}

// bareKeyOK reports whether a name can be written unquoted in a table header.
func bareKeyOK(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func tomlKey(s string) string {
	if bareKeyOK(s) {
		return s
	}
	return strconv.Quote(s)
}

func tomlValue(s string) string {
	return strconv.Quote(s)
}
