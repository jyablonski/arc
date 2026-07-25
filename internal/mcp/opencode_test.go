package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpencodeWrite_TranslatesSchema(t *testing.T) {
	paths := newTestPaths(t)
	p := &OpencodeProvider{Path: paths.OpencodeConfig}

	require.NoError(t, p.Write(map[string]Server{
		"ctx7": {Type: TypeStdio, Command: "uvx", Args: []string{"context7-mcp", "--v"},
			Env: map[string]string{"LOG": "ERROR"}},
		"remote": {Type: TypeHTTP, URL: "https://example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer {env:TOK}"}},
	}, nil))

	out := readFile(t, paths.OpencodeConfig)
	// stdio becomes local with one argv array; http becomes remote.
	assert.Contains(t, out, `"type": "local"`)
	assert.Contains(t, out, `"uvx"`)
	assert.Contains(t, out, `"context7-mcp"`)
	assert.Contains(t, out, `"environment"`)
	assert.Contains(t, out, `"type": "remote"`)
	assert.Contains(t, out, `"$schema"`)
	assert.NotContains(t, out, `"args"`, "opencode folds args into the command array")
}

func TestOpencodeRoundTrip(t *testing.T) {
	paths := newTestPaths(t)
	p := &OpencodeProvider{Path: paths.OpencodeConfig}

	in := map[string]Server{
		"local-srv":  {Type: TypeStdio, Command: "uvx", Args: []string{"pkg"}},
		"bare-srv":   {Type: TypeStdio, Command: "npx"},
		"remote-srv": {Type: TypeHTTP, URL: "https://example.com/mcp"},
		"off-srv":    {Type: TypeStdio, Command: "x", Enabled: boolPtr(false)},
	}
	require.NoError(t, p.Write(in, nil))

	got, err := p.Read()
	require.NoError(t, err)
	require.Len(t, got, len(in))
	for name, want := range in {
		assert.True(t, p.Normalize(want).Equivalent(got[name]),
			"%s did not round-trip: wrote %+v, read %+v", name, want, got[name])
	}
	// opencode has a real disable flag, so a disabled server stays in the file.
	assert.Contains(t, got, "off-srv")
	assert.False(t, got["off-srv"].IsEnabled())
}

// opencode has no separate SSE transport: it collapses into remote. Normalize
// has to report that, or every sync would show permanent drift.
func TestOpencodeNormalize_CollapsesSSE(t *testing.T) {
	p := &OpencodeProvider{Path: "/nonexistent"}
	got := p.Normalize(Server{Type: TypeSSE, URL: "https://example.com/sse"})
	assert.Equal(t, TypeHTTP, got.EffectiveType())
	assert.Equal(t, "https://example.com/sse", got.URL)
}

func TestOpencodeWrite_PreservesUnrelatedConfig(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.OpencodeConfig, `{
  "$schema": "https://opencode.ai/config.json",
  "theme": "tokyonight",
  "model": "anthropic/claude-opus-5",
  "mcp": {
    "mine": { "type": "local", "command": ["my-server"] }
  }
}`)
	p := &OpencodeProvider{Path: paths.OpencodeConfig}

	require.NoError(t, p.Write(map[string]Server{"shared": {Type: TypeStdio, Command: "uvx"}}, nil))

	out := readFile(t, paths.OpencodeConfig)
	assert.Contains(t, out, `"theme": "tokyonight"`)
	assert.Contains(t, out, `"model": "anthropic/claude-opus-5"`)

	got, err := p.Read()
	require.NoError(t, err)
	assert.Contains(t, got, "mine")
	assert.Contains(t, got, "shared")
}

func TestOpencodeRead_MarksUnmodeledFields(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.OpencodeConfig, `{
  "mcp": {
    "custom": {
      "type": "remote",
      "url": "https://example.com/mcp",
      "timeout": 15000
    }
  }
}`)

	servers, err := (&OpencodeProvider{Path: paths.OpencodeConfig}).Read()
	require.NoError(t, err)
	assert.True(t, servers["custom"].unmodeled)
}
