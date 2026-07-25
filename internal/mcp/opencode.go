package mcp

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jyablonski/arc/internal/filemode"
)

// OpencodeProvider syncs into the "mcp" block of opencode.json. opencode is the
// only provider with a materially different schema: servers are local/remote
// rather than stdio/http/sse, the command and its arguments are one argv array,
// and env is spelled "environment".
type OpencodeProvider struct{ Path string }

const opencodeSchema = "https://opencode.ai/config.json"

type opencodeServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

func (p *OpencodeProvider) Name() string       { return "opencode" }
func (p *OpencodeProvider) ConfigPath() string { return p.Path }

// opencode has a native enabled flag, so a disabled server stays in its config.
func (p *OpencodeProvider) OmitsDisabled() bool { return false }

func (p *OpencodeProvider) Supports(name string, s Server) error {
	if err := Validate(name, s); err != nil {
		return err
	}
	// opencode's "remote" transport is streamable HTTP; it has no separate SSE
	// mode, but an SSE endpoint is reachable over the same config shape, so
	// both map to remote.
	return nil
}

// Normalize round-trips through opencode's schema, which is where sse collapses
// into the remote transport and command+args collapse into one argv array.
func (p *OpencodeProvider) Normalize(s Server) Server {
	return renderOpencodeServer(s).toCanonical()
}

func (p *OpencodeProvider) Read() (map[string]Server, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Server{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", p.Path, err)
	}
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", p.Path, err)
	}
	raw, ok := root.get("mcp")
	if !ok {
		return map[string]Server{}, nil
	}
	entries, err := decodeJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s mcp block: %w", p.Path, err)
	}
	out := make(map[string]Server, entries.len())
	for _, name := range entries.keys {
		entryRaw, _ := entries.get(name)
		var entry opencodeServer
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			return nil, fmt.Errorf("parse %s mcp.%s: %w", p.Path, name, err)
		}
		entryObject, err := decodeJSONObject(entryRaw)
		if err != nil {
			return nil, fmt.Errorf("parse %s mcp.%s: %w", p.Path, name, err)
		}
		server := entry.toCanonical()
		for _, key := range entryObject.keys {
			if !opencodeServerField(key) {
				server.unmodeled = true
				break
			}
		}
		out[name] = server
	}
	return out, nil
}

func opencodeServerField(key string) bool {
	switch key {
	case "type", "command", "environment", "url", "headers", "enabled":
		return true
	default:
		return false
	}
}

func (e opencodeServer) toCanonical() Server {
	s := Server{Enabled: e.Enabled}
	if e.Type == "remote" {
		s.Type = TypeHTTP
		s.URL = e.URL
		s.Headers = e.Headers
		return s
	}
	s.Type = TypeStdio
	s.Env = e.Environment
	if len(e.Command) > 0 {
		s.Command = e.Command[0]
		if len(e.Command) > 1 {
			s.Args = e.Command[1:]
		}
	}
	return s
}

func renderOpencodeServer(s Server) opencodeServer {
	out := opencodeServer{}
	if !s.IsEnabled() {
		disabled := false
		out.Enabled = &disabled
	}
	if s.EffectiveType() == TypeStdio {
		out.Type = "local"
		out.Command = append([]string{s.Command}, s.Args...)
		out.Environment = s.Env
		return out
	}
	out.Type = "remote"
	out.URL = s.URL
	out.Headers = s.Headers
	return out
}

func (p *OpencodeProvider) Write(servers map[string]Server, owned []string) error {
	data, err := os.ReadFile(p.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", p.Path, err)
	}
	root, err := decodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", p.Path, err)
	}

	entries := newJSONObject()
	if raw, ok := root.get("mcp"); ok {
		entries, err = decodeJSONObject(raw)
		if err != nil {
			return fmt.Errorf("parse %s mcp block: %w", p.Path, err)
		}
	}

	for _, name := range owned {
		entries.delete(name)
	}
	for _, name := range SortedNames(servers) {
		raw, err := json.Marshal(renderOpencodeServer(servers[name]))
		if err != nil {
			return err
		}
		entries.set(name, raw)
	}

	if _, keyExisted := root.get("mcp"); entries.len() == 0 && !keyExisted {
		return nil
	}

	// A brand-new opencode.json leads with $schema so editor completion works;
	// set it before mcp so it lands first in key order.
	if _, ok := root.get("$schema"); !ok && root.len() == 0 {
		schema, _ := json.Marshal(opencodeSchema)
		root.set("$schema", schema)
	}

	encoded, err := entries.encode()
	if err != nil {
		return err
	}
	root.set("mcp", encoded)

	out, err := root.encode()
	if err != nil {
		return err
	}
	return writeFileAtomic(p.Path, out, filemode.Private)
}
