package mcp

import (
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/jyablonski/arc/internal/filemode"
)

// CodexProvider syncs into the [mcp_servers.*] tables of ~/.codex/config.toml.
//
// Codex has no SSE transport, but streamable HTTP supports static headers,
// environment-backed headers, and a dedicated bearer-token environment field.
// Supports rejects references that cannot be represented instead of writing
// config that would silently not work.
type CodexProvider struct{ Path string }

const codexTable = "mcp_servers"

func (p *CodexProvider) Name() string       { return "codex" }
func (p *CodexProvider) ConfigPath() string { return p.Path }

// Codex has a native enabled flag, so a disabled server stays in its config.
func (p *CodexProvider) OmitsDisabled() bool { return false }

func (p *CodexProvider) Supports(name string, s Server) error {
	if err := Validate(name, s); err != nil {
		return err
	}
	if s.EffectiveType() == TypeSSE {
		return fmt.Errorf("codex has no sse transport")
	}
	if envRefRe.MatchString(s.URL) {
		return fmt.Errorf("codex cannot expand environment references in an MCP URL")
	}
	if s.EffectiveType() == TypeStdio {
		if envRefRe.MatchString(s.Command) {
			return fmt.Errorf("codex cannot expand environment references in an MCP command")
		}
		for _, arg := range s.Args {
			if envRefRe.MatchString(arg) {
				return fmt.Errorf("codex cannot expand environment references in MCP arguments")
			}
		}
		for key, value := range s.Env {
			if !envRefRe.MatchString(value) {
				continue
			}
			ref, ok := exactEnvRef(value)
			if !ok || ref != key {
				return fmt.Errorf("codex can inherit only a same-name stdio environment variable, not %s=%q", key, value)
			}
		}
		return nil
	}
	for key, value := range s.Headers {
		if strings.EqualFold(key, "authorization") {
			if _, ok := bearerEnvVar(map[string]string{key: value}); ok {
				continue
			}
		}
		if !envRefRe.MatchString(value) {
			continue
		}
		if _, ok := exactEnvRef(value); !ok {
			return fmt.Errorf("codex requires header %q to be a literal or exactly {env:VAR}", key)
		}
	}
	return nil
}

// Normalize mirrors what Read returns after Write: Codex keeps the transport,
// command, args, env, and a bearer token as an env var name, and drops
// everything else. Supports has already rejected anything it cannot hold.
func (p *CodexProvider) Normalize(s Server) Server {
	out := Server{Type: s.EffectiveType(), Enabled: s.Enabled}
	if out.Type == TypeStdio {
		out.Command, out.Args, out.Env = s.Command, s.Args, s.Env
		return out
	}
	out.URL = s.URL
	if len(s.Headers) > 0 {
		out.Headers = make(map[string]string, len(s.Headers))
		maps.Copy(out.Headers, s.Headers)
	}
	return out
}

func (p *CodexProvider) Read() (map[string]Server, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Server{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", p.Path, err)
	}
	doc := parseTOMLDoc(data)

	out := map[string]Server{}
	// Sub-tables ([mcp_servers.foo.env]) are collected separately and folded in
	// after, since they may appear before or after their parent's keys.
	envs := map[string]map[string]string{}
	httpHeaders := map[string]map[string]string{}
	envHTTPHeaders := map[string]map[string]string{}
	unmodeled := map[string]bool{}

	for _, sec := range doc.sections {
		if len(sec.path) < 2 || sec.path[0] != codexTable {
			continue
		}
		name := sec.path[1]
		if sec.array {
			unmodeled[name] = true
			continue
		}
		kv := parseTOMLKeyValues(doc.body(sec))
		if len(sec.path) == 3 {
			var target map[string]map[string]string
			switch sec.path[2] {
			case "env":
				target = envs
			case "http_headers":
				target = httpHeaders
			case "env_http_headers":
				target = envHTTPHeaders
			default:
				unmodeled[name] = true
				continue
			}
			values := map[string]string{}
			for k, raw := range kv {
				if v, ok := tomlUnquote(raw); ok {
					values[k] = v
				}
			}
			target[name] = values
			continue
		}
		if len(sec.path) != 2 {
			unmodeled[name] = true
			continue
		}
		server := codexToCanonical(kv)
		for key := range kv {
			if !codexServerField(key) {
				server.unmodeled = true
				break
			}
		}
		out[name] = server
	}

	for name, env := range envs {
		s, ok := out[name]
		if !ok {
			continue
		}
		if s.Env == nil {
			s.Env = map[string]string{}
		}
		maps.Copy(s.Env, env)
		out[name] = s
	}
	for name, headers := range httpHeaders {
		s, ok := out[name]
		if !ok {
			continue
		}
		if s.Headers == nil {
			s.Headers = map[string]string{}
		}
		maps.Copy(s.Headers, headers)
		out[name] = s
	}
	for name, headers := range envHTTPHeaders {
		s, ok := out[name]
		if !ok {
			continue
		}
		if s.Headers == nil {
			s.Headers = map[string]string{}
		}
		for header, envVar := range headers {
			s.Headers[header] = fmt.Sprintf("{env:%s}", envVar)
		}
		out[name] = s
	}
	for name := range unmodeled {
		s, ok := out[name]
		if !ok {
			continue
		}
		s.unmodeled = true
		out[name] = s
	}
	return out, nil
}

func codexToCanonical(kv map[string]string) Server {
	var s Server
	if raw, ok := kv["url"]; ok {
		if v, ok := tomlUnquote(raw); ok {
			s.URL = v
		}
	}
	if raw, ok := kv["command"]; ok {
		if v, ok := tomlUnquote(raw); ok {
			s.Command = v
		}
	}
	if raw, ok := kv["args"]; ok {
		if v, ok := tomlStringArray(raw); ok {
			s.Args = v
		}
	}
	if raw, ok := kv["env"]; ok {
		if v, ok := tomlStringTable(raw); ok && len(v) > 0 {
			s.Env = v
		}
	}
	if raw, ok := kv["env_vars"]; ok {
		if values, ok := tomlStringArray(raw); ok {
			if s.Env == nil {
				s.Env = map[string]string{}
			}
			for _, value := range values {
				s.Env[value] = fmt.Sprintf("{env:%s}", value)
			}
		} else {
			s.unmodeled = true
		}
	}
	if raw, ok := kv["bearer_token_env_var"]; ok {
		if v, ok := tomlUnquote(raw); ok && v != "" {
			s.Headers = map[string]string{"Authorization": fmt.Sprintf("Bearer {env:%s}", v)}
		}
	}
	if raw, ok := kv["http_headers"]; ok {
		if headers, ok := tomlStringTable(raw); ok {
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			maps.Copy(s.Headers, headers)
		} else {
			s.unmodeled = true
		}
	}
	if raw, ok := kv["env_http_headers"]; ok {
		if headers, ok := tomlStringTable(raw); ok {
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			for header, envVar := range headers {
				s.Headers[header] = fmt.Sprintf("{env:%s}", envVar)
			}
		} else {
			s.unmodeled = true
		}
	}
	if raw, ok := kv["enabled"]; ok {
		if v, ok := tomlBool(raw); ok {
			s.Enabled = &v
		}
	}
	s.Type = s.EffectiveType()
	return s
}

func codexServerField(key string) bool {
	switch key {
	case "url", "command", "args", "env", "env_vars", "bearer_token_env_var",
		"http_headers", "env_http_headers", "enabled":
		return true
	default:
		return false
	}
}

// renderCodexServer writes one [mcp_servers.name] table.
func renderCodexServer(name string, s Server) []string {
	lines := []string{fmt.Sprintf("[%s.%s]", codexTable, tomlKey(name))}
	if s.EffectiveType() == TypeStdio {
		lines = append(lines, fmt.Sprintf("command = %s", tomlValue(s.Command)))
		if len(s.Args) > 0 {
			quoted := make([]string, 0, len(s.Args))
			for _, a := range s.Args {
				quoted = append(quoted, tomlValue(a))
			}
			lines = append(lines, fmt.Sprintf("args = [%s]", strings.Join(quoted, ", ")))
		}
		if len(s.Env) > 0 {
			literalEnv := map[string]string{}
			var inheritedEnv []string
			for _, k := range sortedKeys(s.Env) {
				if ref, ok := exactEnvRef(s.Env[k]); ok && ref == k {
					inheritedEnv = append(inheritedEnv, k)
					continue
				}
				literalEnv[k] = s.Env[k]
			}
			if len(literalEnv) > 0 {
				lines = append(lines, fmt.Sprintf("env = %s", renderTOMLStringTable(literalEnv)))
			}
			if len(inheritedEnv) > 0 {
				quoted := make([]string, 0, len(inheritedEnv))
				for _, name := range inheritedEnv {
					quoted = append(quoted, tomlValue(name))
				}
				lines = append(lines, fmt.Sprintf("env_vars = [%s]", strings.Join(quoted, ", ")))
			}
		}
	} else {
		lines = append(lines, fmt.Sprintf("url = %s", tomlValue(s.URL)))
		staticHeaders := map[string]string{}
		envHeaders := map[string]string{}
		for _, key := range sortedKeys(s.Headers) {
			value := s.Headers[key]
			if strings.EqualFold(key, "authorization") {
				if variable, ok := bearerEnvVar(map[string]string{key: value}); ok {
					lines = append(lines, fmt.Sprintf("bearer_token_env_var = %s", tomlValue(variable)))
					continue
				}
			}
			if variable, ok := exactEnvRef(value); ok {
				envHeaders[key] = variable
				continue
			}
			staticHeaders[key] = value
		}
		if len(staticHeaders) > 0 {
			lines = append(lines, fmt.Sprintf("http_headers = %s", renderTOMLStringTable(staticHeaders)))
		}
		if len(envHeaders) > 0 {
			lines = append(lines, fmt.Sprintf("env_http_headers = %s", renderTOMLStringTable(envHeaders)))
		}
	}
	if !s.IsEnabled() {
		lines = append(lines, "enabled = false")
	}
	return lines
}

func renderTOMLStringTable(values map[string]string) string {
	pairs := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		pairs = append(pairs, fmt.Sprintf("%s = %s", tomlKey(key), tomlValue(values[key])))
	}
	return fmt.Sprintf("{ %s }", strings.Join(pairs, ", "))
}

func exactEnvRef(value string) (string, bool) {
	matches := envRefRe.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil || matches[0] != strings.TrimSpace(value) {
		return "", false
	}
	return matches[1], true
}

func (p *CodexProvider) Write(servers map[string]Server, owned []string) error {
	data, err := os.ReadFile(p.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", p.Path, err)
	}
	doc := parseTOMLDoc(data)

	ownedSet := map[string]bool{}
	for _, n := range owned {
		ownedSet[n] = true
	}
	for n := range servers {
		ownedSet[n] = true
	}

	// Mark every line belonging to a table arc owns, including sub-tables, then
	// rebuild the file without them. Comments above a header are left in place:
	// deleting a user's comment is worse than orphaning one.
	drop := make([]bool, len(doc.lines))
	dropped := false
	for _, sec := range doc.sections {
		if sec.array || len(sec.path) < 2 || sec.path[0] != codexTable || !ownedSet[sec.path[1]] {
			continue
		}
		for i := sec.start; i < sec.end && i < len(drop); i++ {
			drop[i] = true
		}
		dropped = true
	}

	kept := make([]string, 0, len(doc.lines))
	for i, ln := range doc.lines {
		if !drop[i] {
			kept = append(kept, ln)
		}
	}
	if dropped {
		kept = trimTrailingBlank(kept)
	}

	names := SortedNames(servers)
	if len(names) == 0 && !dropped {
		return nil
	}

	for _, name := range names {
		if len(kept) > 0 {
			kept = append(kept, "")
		}
		kept = append(kept, renderCodexServer(name, servers[name])...)
	}

	out := (&tomlDoc{lines: kept}).render()
	return writeFileAtomic(p.Path, []byte(out), filemode.Private)
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
