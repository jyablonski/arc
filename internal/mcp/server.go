package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jyablonski/arc/internal/filemode"
)

type ServerType string

const (
	TypeStdio ServerType = "stdio"
	TypeHTTP  ServerType = "http"
	TypeSSE   ServerType = "sse"
)

// Server is one MCP server in arc's canonical dialect, which is Claude Code's
// and Cursor's shape: those two need no translation, and it is the most
// expressive of the four (Codex and opencode are both strict subsets plus a
// rename).
type Server struct {
	Type    ServerType        `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// Enabled is a pointer so an absent key means enabled. Providers with a
	// native disable flag get enabled=false; providers without one (Claude,
	// Cursor) omit the server entirely rather than write a key their schema
	// may reject.
	Enabled *bool `json:"enabled,omitempty"`

	// Providers restricts the server to a subset of provider names. Empty
	// means every provider that can express it.
	Providers []string `json:"providers,omitempty"`

	// unmodeled is set only on entries read from a provider when that entry
	// contains fields arc cannot round-trip. Such entries must remain conflicts
	// unless the user explicitly forces an overwrite.
	unmodeled bool
}

// File is the canonical ~/ai/mcp.json document.
type File struct {
	MCPServers map[string]Server `json:"mcpServers"`
}

func (s Server) IsEnabled() bool { return s.Enabled == nil || *s.Enabled }

func (s Server) AppliesTo(provider string) bool {
	if len(s.Providers) == 0 {
		return true
	}
	for _, p := range s.Providers {
		if strings.EqualFold(strings.TrimSpace(p), provider) {
			return true
		}
	}
	return false
}

// EffectiveType infers the transport when the canonical file omits it, so a
// hand-written entry with just a url or just a command still works.
func (s Server) EffectiveType() ServerType {
	if s.Type != "" {
		return s.Type
	}
	if strings.TrimSpace(s.URL) != "" {
		return TypeHTTP
	}
	return TypeStdio
}

// Equivalent reports whether two servers describe the same thing, ignoring
// canonical-only bookkeeping (Providers) that never reaches a provider file.
// Sync uses it to adopt a hand-configured server that already matches instead
// of reporting a spurious conflict.
func (s Server) Equivalent(other Server) bool {
	if s.EffectiveType() != other.EffectiveType() {
		return false
	}
	if s.Command != other.Command || s.URL != other.URL {
		return false
	}
	if s.IsEnabled() != other.IsEnabled() {
		return false
	}
	if !equalSlices(s.Args, other.Args) {
		return false
	}
	return equalMaps(s.Env, other.Env) && equalMaps(s.Headers, other.Headers)
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || av != bv {
			return false
		}
	}
	return true
}

// envRefRe matches the {env:VAR} indirection Claude, Cursor, and opencode all
// understand. It is the only way canonical config is allowed to reference a
// credential.
var envRefRe = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// EnvRefs returns every environment variable the server references, sorted.
func (s Server) EnvRefs() []string {
	seen := map[string]bool{}
	scan := func(v string) {
		for _, m := range envRefRe.FindAllStringSubmatch(v, -1) {
			seen[m[1]] = true
		}
	}
	scan(s.URL)
	scan(s.Command)
	for _, a := range s.Args {
		scan(a)
	}
	for _, v := range s.Env {
		scan(v)
	}
	for _, v := range s.Headers {
		scan(v)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateName rejects names that cannot round-trip through every provider's
// file format.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("server name %q must match %s", name, nameRe.String())
	}
	return nil
}

// Validate checks one server for internal consistency and for inline secrets.
func Validate(name string, s Server) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	switch s.EffectiveType() {
	case TypeStdio:
		if strings.TrimSpace(s.Command) == "" {
			return fmt.Errorf("stdio server %q needs a command", name)
		}
		if strings.TrimSpace(s.URL) != "" {
			return fmt.Errorf("stdio server %q must not set url", name)
		}
	case TypeHTTP, TypeSSE:
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("%s server %q needs a url", s.EffectiveType(), name)
		}
		if strings.TrimSpace(s.Command) != "" {
			return fmt.Errorf("%s server %q must not set command", s.EffectiveType(), name)
		}
	default:
		return fmt.Errorf("server %q has unknown type %q (want stdio, http, or sse)", name, s.Type)
	}
	return checkInlineSecrets(name, s)
}

// secretishKey matches env/header names that conventionally hold a credential.
var secretishKey = regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|api[_-]?key|auth)`)

// literalCredential matches well-known credential prefixes, so an obviously
// pasted token is caught even under an innocuous key name.
var literalCredential = regexp.MustCompile(`(?i)^(bearer\s+\S+|sk-[A-Za-z0-9_-]{8,}|ghp_[A-Za-z0-9]{8,}|github_pat_[A-Za-z0-9_]{8,}|xox[baprs]-[A-Za-z0-9-]{8,}|AKIA[A-Z0-9]{12,})$`)

// checkInlineSecrets keeps literal credentials out of the canonical file, which
// is meant to be dotfile-committable. Credentials belong in {env:VAR}.
func checkInlineSecrets(name string, s Server) error {
	check := func(kind, key, value string) error {
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		withoutRefs := strings.TrimSpace(envRefRe.ReplaceAllString(v, ""))
		if envRefRe.MatchString(v) && safeCredentialReference(kind, key, withoutRefs) {
			return nil
		}
		if literalCredential.MatchString(v) ||
			literalCredential.MatchString(withoutRefs) ||
			secretishKey.MatchString(key) {
			return fmt.Errorf(
				"server %q: %s %q looks like an inline credential; use {env:VAR_NAME} instead",
				name, kind, key)
		}
		return nil
	}
	for _, k := range sortedKeys(s.Headers) {
		if err := check("header", k, s.Headers[k]); err != nil {
			return err
		}
	}
	for _, k := range sortedKeys(s.Env) {
		if err := check("env", k, s.Env[k]); err != nil {
			return err
		}
	}
	if literalCredential.MatchString(strings.TrimSpace(s.Command)) {
		return fmt.Errorf("server %q: command looks like an inline credential; use {env:VAR_NAME} instead", name)
	}
	for i, arg := range s.Args {
		trimmed := strings.TrimSpace(arg)
		if key, value, ok := strings.Cut(strings.TrimLeft(trimmed, "-"), "="); ok && secretishKey.MatchString(key) {
			if err := check("argument", key, value); err != nil {
				return err
			}
		}
		if literalCredential.MatchString(trimmed) {
			return fmt.Errorf("server %q: argument %d looks like an inline credential; use {env:VAR_NAME} instead", name, i+1)
		}
		if i > 0 {
			key := strings.TrimLeft(strings.TrimSpace(s.Args[i-1]), "-")
			if secretishKey.MatchString(key) {
				if err := check("argument", key, arg); err != nil {
					return err
				}
			}
		}
	}
	if parsed, err := url.Parse(s.URL); err == nil {
		if parsed.User != nil {
			if password, ok := parsed.User.Password(); ok && password != "" {
				return fmt.Errorf("server %q: url contains an inline password; use {env:VAR_NAME} instead", name)
			}
		}
		query := parsed.Query()
		queryKeys := make([]string, 0, len(query))
		for key := range query {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)
		for _, key := range queryKeys {
			for _, value := range query[key] {
				if err := check("url query parameter", key, value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func safeCredentialReference(kind, key, withoutRefs string) bool {
	if withoutRefs == "" {
		return true
	}
	return kind == "header" &&
		strings.EqualFold(key, "authorization") &&
		strings.EqualFold(withoutRefs, "bearer")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Load reads the canonical file. A missing file is not an error: it yields an
// empty set so `list`, `sync`, and `import` all work on a fresh machine.
func Load(path string) (File, error) {
	f := File{MCPServers: map[string]Server{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.MCPServers == nil {
		f.MCPServers = map[string]Server{}
	}
	return f, nil
}

func Save(path string, f File) error {
	if f.MCPServers == nil {
		f.MCPServers = map[string]Server{}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// Canonical is deliberately world-readable: it holds no credentials by
	// construction and is meant to live in a dotfile repo.
	return writeFileAtomic(path, append(data, '\n'), filemode.File)
}

// SortedNames returns canonical server names in stable order.
func SortedNames(servers map[string]Server) []string {
	out := make([]string, 0, len(servers))
	for k := range servers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
