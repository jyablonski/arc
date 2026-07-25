package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/jyablonski/arc/internal/filemode"
)

// bearerEnvRe matches an Authorization header whose value is nothing but a
// bearer token read from the environment.
var bearerEnvRe = regexp.MustCompile(`^\s*[Bb]earer\s+\{env:([A-Za-z_][A-Za-z0-9_]*)\}\s*$`)

// jsonDialect implements the Claude/Cursor shape: an "mcpServers" object whose
// entries are already arc's canonical form, embedded in a JSON file that may
// hold unrelated keys. Merging goes through jsonObject so those keys — and
// their order — survive untouched.
type jsonDialect struct {
	name      string
	path      string
	key       string
	envSyntax jsonEnvSyntax
}

func (d *jsonDialect) Name() string       { return d.name }
func (d *jsonDialect) ConfigPath() string { return d.path }

type jsonEnvSyntax int

const (
	jsonEnvClaude jsonEnvSyntax = iota
	jsonEnvCursor
)

var (
	claudeEnvRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	cursorEnvRefRe = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// Supports accepts everything: the canonical dialect is this dialect. The one
// exception is a disabled server, which these tools have no file-level flag
// for; Write omits those instead of inventing a key.
func (d *jsonDialect) Supports(name string, s Server) error {
	return Validate(name, s)
}

func (d *jsonDialect) Read() (map[string]Server, error) {
	data, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Server{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", d.path, err)
	}
	obj, err := decodeJSONObject(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", d.path, err)
	}
	raw, ok := obj.get(d.key)
	if !ok {
		return map[string]Server{}, nil
	}
	entries, err := decodeJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s.%s: %w", d.path, d.key, err)
	}
	servers := make(map[string]Server, entries.len())
	for _, name := range entries.keys {
		entryRaw, _ := entries.get(name)
		var s Server
		if err := json.Unmarshal(entryRaw, &s); err != nil {
			return nil, fmt.Errorf("parse %s.%s.%s: %w", d.path, d.key, name, err)
		}
		entry, err := decodeJSONObject(entryRaw)
		if err != nil {
			return nil, fmt.Errorf("parse %s.%s.%s: %w", d.path, d.key, name, err)
		}
		for _, key := range entry.keys {
			if !jsonServerField(key) {
				s.unmodeled = true
				break
			}
		}
		if s.Type == "streamable-http" {
			s.Type = TypeHTTP
		}
		s.Type = s.EffectiveType()
		s = translateJSONServerRefs(s, d.envSyntax, false)
		servers[name] = s
	}
	return servers, nil
}

func (d *jsonDialect) Write(servers map[string]Server, owned []string) error {
	data, err := os.ReadFile(d.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", d.path, err)
	}
	root, err := decodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", d.path, err)
	}

	entries := newJSONObject()
	if raw, ok := root.get(d.key); ok {
		entries, err = decodeJSONObject(raw)
		if err != nil {
			return fmt.Errorf("parse %s.%s: %w", d.path, d.key, err)
		}
	}

	// Drop everything arc owns first, so a server that moved out of canonical
	// (or became disabled) disappears rather than lingering.
	for _, name := range owned {
		entries.delete(name)
	}
	for _, name := range SortedNames(servers) {
		s := servers[name]
		if !s.IsEnabled() {
			continue
		}
		raw, err := json.Marshal(renderJSONServer(translateJSONServerRefs(s, d.envSyntax, true)))
		if err != nil {
			return err
		}
		entries.set(name, raw)
	}

	// Nothing to add and no block to clean up: leave the file alone rather than
	// creating one (or adding a key) just to hold an empty object.
	if _, keyExisted := root.get(d.key); entries.len() == 0 && !keyExisted {
		return nil
	}

	encodedEntries, err := entries.encode()
	if err != nil {
		return err
	}
	root.set(d.key, encodedEntries)

	out, err := root.encode()
	if err != nil {
		return err
	}
	return writeFileAtomic(d.path, out, filemode.Private)
}

// jsonServer is the on-disk entry shape. It is Server minus the canonical-only
// fields (enabled, providers) that these tools would not understand.
type jsonServer struct {
	Type    ServerType        `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// normalizeJSONServer is what Read returns after Write: the canonical dialect
// is this dialect, so only the canonical-only fields drop away.
func normalizeJSONServer(s Server) Server {
	j := renderJSONServer(s)
	return Server{
		Type:    j.Type,
		Command: j.Command,
		Args:    j.Args,
		Env:     j.Env,
		URL:     j.URL,
		Headers: j.Headers,
	}
}

func renderJSONServer(s Server) jsonServer {
	out := jsonServer{Type: s.EffectiveType()}
	switch out.Type {
	case TypeStdio:
		out.Command = s.Command
		out.Args = s.Args
		out.Env = s.Env
	default:
		out.URL = s.URL
		out.Headers = s.Headers
	}
	return out
}

func jsonServerField(key string) bool {
	switch key {
	case "type", "command", "args", "env", "url", "headers":
		return true
	default:
		return false
	}
}

func translateJSONServerRefs(s Server, syntax jsonEnvSyntax, outbound bool) Server {
	translate := func(value string) string {
		switch {
		case outbound && syntax == jsonEnvClaude:
			return envRefRe.ReplaceAllString(value, `${$1}`)
		case outbound && syntax == jsonEnvCursor:
			return envRefRe.ReplaceAllString(value, `${env:$1}`)
		case !outbound && syntax == jsonEnvClaude:
			return claudeEnvRefRe.ReplaceAllString(value, `{env:$1}`)
		case !outbound && syntax == jsonEnvCursor:
			return cursorEnvRefRe.ReplaceAllString(value, `{env:$1}`)
		default:
			return value
		}
	}
	s.Args = append([]string(nil), s.Args...)
	if s.Env != nil {
		env := make(map[string]string, len(s.Env))
		for key, value := range s.Env {
			env[key] = value
		}
		s.Env = env
	}
	if s.Headers != nil {
		headers := make(map[string]string, len(s.Headers))
		for key, value := range s.Headers {
			headers[key] = value
		}
		s.Headers = headers
	}
	s.Command = translate(s.Command)
	s.URL = translate(s.URL)
	for i := range s.Args {
		s.Args[i] = translate(s.Args[i])
	}
	for key, value := range s.Env {
		s.Env[key] = translate(value)
	}
	for key, value := range s.Headers {
		s.Headers[key] = translate(value)
	}
	return s
}

// ClaudeProvider syncs into the user-scope mcpServers block of ~/.claude.json,
// the same place `claude mcp add -s user` writes. The rest of that file is
// session state and is preserved byte-for-byte.
type ClaudeProvider struct{ Path string }

func (p *ClaudeProvider) dialect() *jsonDialect {
	return &jsonDialect{name: "claude", path: p.Path, key: "mcpServers", envSyntax: jsonEnvClaude}
}

func (p *ClaudeProvider) Name() string                      { return "claude" }
func (p *ClaudeProvider) ConfigPath() string                { return p.Path }
func (p *ClaudeProvider) Supports(n string, s Server) error { return p.dialect().Supports(n, s) }
func (p *ClaudeProvider) Normalize(s Server) Server         { return normalizeJSONServer(s) }
func (p *ClaudeProvider) OmitsDisabled() bool               { return true }
func (p *ClaudeProvider) Read() (map[string]Server, error)  { return p.dialect().Read() }
func (p *ClaudeProvider) Write(servers map[string]Server, owned []string) error {
	return p.dialect().Write(servers, owned)
}

// CursorProvider syncs into ~/.cursor/mcp.json, which uses Claude's schema.
type CursorProvider struct{ Path string }

func (p *CursorProvider) dialect() *jsonDialect {
	return &jsonDialect{name: "cursor", path: p.Path, key: "mcpServers", envSyntax: jsonEnvCursor}
}

func (p *CursorProvider) Name() string                      { return "cursor" }
func (p *CursorProvider) ConfigPath() string                { return p.Path }
func (p *CursorProvider) Supports(n string, s Server) error { return p.dialect().Supports(n, s) }
func (p *CursorProvider) Normalize(s Server) Server         { return normalizeJSONServer(s) }
func (p *CursorProvider) OmitsDisabled() bool               { return true }
func (p *CursorProvider) Read() (map[string]Server, error)  { return p.dialect().Read() }
func (p *CursorProvider) Write(servers map[string]Server, owned []string) error {
	return p.dialect().Write(servers, owned)
}
