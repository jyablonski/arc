package mcp

import (
	"fmt"
	"strings"
)

// Provider is one AI tool arc can sync MCP servers into.
//
// Read/Write operate on the whole MCP section of the tool's config; the caller
// (Manager) decides which servers it owns and merges accordingly, so a provider
// implementation never has to know about ownership or conflicts.
type Provider interface {
	Name() string
	// ConfigPath is the file the provider's servers live in, shown in output.
	ConfigPath() string
	// Supports reports why a server cannot be expressed in this provider's
	// dialect, or nil when it translates cleanly.
	Supports(name string, s Server) error
	// Normalize returns the canonical form the provider will hold after a
	// write. Translation is lossy in places (opencode collapses sse into its
	// remote transport), so comparing a provider's file against raw canonical
	// would report permanent drift; comparing against this does not.
	Normalize(s Server) Server
	// Read returns the servers currently configured in the provider's file. A
	// missing file yields an empty map, not an error.
	Read() (map[string]Server, error)
	// Write merges servers into the provider's file, replacing exactly the
	// entries named in owned and leaving every other key untouched.
	Write(servers map[string]Server, owned []string) error
	// OmitsDisabled reports that this provider has no file-level disable flag,
	// so a disabled server is left out of its config entirely rather than
	// written with a key its schema may reject.
	OmitsDisabled() bool
}

// DefaultProviders returns the provider set in the same order as the rest of
// the CLI reports them.
func DefaultProviders(p Paths) []Provider {
	return []Provider{
		&ClaudeProvider{Path: p.ClaudeJSON},
		&CodexProvider{Path: p.CodexConfig},
		&CursorProvider{Path: p.CursorMCP},
		&OpencodeProvider{Path: p.OpencodeConfig},
	}
}

// ProviderNames lists provider names for filter validation and help text.
func ProviderNames(providers []Provider) []string {
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Name())
	}
	return out
}

// FilterProviders narrows a provider set by name, erroring on an unknown name
// so a typo does not silently sync nothing.
func FilterProviders(providers []Provider, names []string) ([]Provider, error) {
	if len(names) == 0 {
		return providers, nil
	}
	byName := map[string]Provider{}
	for _, p := range providers {
		byName[p.Name()] = p
	}
	var out []Provider
	for _, n := range names {
		p, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q (want one of %v)", n, ProviderNames(providers))
		}
		out = append(out, p)
	}
	return out, nil
}

// bearerEnvVar reports the variable name when headers are exactly a bearer
// token sourced from the environment. Codex gives this common case a dedicated
// bearer_token_env_var field.
func bearerEnvVar(headers map[string]string) (string, bool) {
	if len(headers) != 1 {
		return "", false
	}
	for k, v := range headers {
		if !strings.EqualFold(k, "authorization") {
			return "", false
		}
		m := bearerEnvRe.FindStringSubmatch(v)
		if m == nil {
			return "", false
		}
		return m[1], true
	}
	return "", false
}
