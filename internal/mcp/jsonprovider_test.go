package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realClaudeJSON mirrors ~/.claude.json: a large state file where mcpServers is
// one key among many, and key order is meaningful to diff noise.
const realClaudeJSON = `{
  "numStartups": 412,
  "installMethod": "native",
  "userID": "abc123",
  "mcpServers": {},
  "projects": {
    "/home/jacob/Documents/arc": {
      "allowedTools": []
    }
  },
  "hasCompletedOnboarding": true
}`

func TestClaudeWrite_PreservesUnrelatedKeysAndOrder(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.ClaudeJSON, realClaudeJSON)
	p := &ClaudeProvider{Path: paths.ClaudeJSON}

	require.NoError(t, p.Write(map[string]Server{
		"homelab": {Type: TypeHTTP, URL: "http://mcp.home/mcp",
			Headers: map[string]string{"Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}"}},
	}, nil))

	out := readFile(t, paths.ClaudeJSON)
	assert.Contains(t, out, `"numStartups": 412`)
	assert.Contains(t, out, `"hasCompletedOnboarding": true`)
	assert.Contains(t, out, `"/home/jacob/Documents/arc"`)

	// Rewriting must not reshuffle ~40 keys of session state on every sync.
	var keys []string
	dec := json.NewDecoder(strings.NewReader(out))
	_, err := dec.Token()
	require.NoError(t, err)
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		require.NoError(t, dec.Decode(&skip))
	}
	assert.Equal(t, []string{
		"numStartups", "installMethod", "userID", "mcpServers", "projects", "hasCompletedOnboarding",
	}, keys)
}

func TestClaudeWrite_RoundTrips(t *testing.T) {
	paths := newTestPaths(t)
	p := &ClaudeProvider{Path: paths.ClaudeJSON}

	in := map[string]Server{
		"ctx7":  {Type: TypeStdio, Command: "uvx", Args: []string{"context7-mcp"}, Env: map[string]string{"L": "1"}},
		"cloud": {Type: TypeHTTP, URL: "https://mcp.cloudflare.com/mcp"},
		"live":  {Type: TypeSSE, URL: "https://mcp.honeycomb.io/mcp"},
	}
	require.NoError(t, p.Write(in, nil))

	got, err := p.Read()
	require.NoError(t, err)
	require.Len(t, got, 3)
	for name, want := range in {
		assert.True(t, p.Normalize(want).Equivalent(got[name]), "%s did not round-trip", name)
	}
	// Unlike Codex, Claude keeps SSE as its own transport.
	assert.Equal(t, TypeSSE, got["live"].EffectiveType())
}

func TestJSONProviders_TranslateEnvironmentReferences(t *testing.T) {
	paths := newTestPaths(t)
	server := map[string]Server{
		"remote": {
			Type: TypeHTTP,
			URL:  "https://{env:HOST}/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer {env:TOKEN}",
			},
		},
	}

	claude := &ClaudeProvider{Path: paths.ClaudeJSON}
	require.NoError(t, claude.Write(server, nil))
	claudeRaw := readFile(t, paths.ClaudeJSON)
	assert.Contains(t, claudeRaw, `${HOST}`)
	assert.Contains(t, claudeRaw, `Bearer ${TOKEN}`)
	assert.NotContains(t, claudeRaw, `{env:`)
	claudeRead, err := claude.Read()
	require.NoError(t, err)
	assert.True(t, server["remote"].Equivalent(claudeRead["remote"]))

	cursor := &CursorProvider{Path: paths.CursorMCP}
	require.NoError(t, cursor.Write(server, nil))
	cursorRaw := readFile(t, paths.CursorMCP)
	assert.Contains(t, cursorRaw, `${env:HOST}`)
	assert.Contains(t, cursorRaw, `Bearer ${env:TOKEN}`)
	cursorRead, err := cursor.Read()
	require.NoError(t, err)
	assert.True(t, server["remote"].Equivalent(cursorRead["remote"]))
}

func TestJSONRead_MarksUnmodeledFields(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.ClaudeJSON, `{
  "mcpServers": {
    "custom": {
      "type": "http",
      "url": "https://example.com/mcp",
      "oauth": {"clientId": "client"}
    }
  }
}`)

	servers, err := (&ClaudeProvider{Path: paths.ClaudeJSON}).Read()
	require.NoError(t, err)
	assert.True(t, servers["custom"].unmodeled)
}

// Claude and Cursor have no file-level disable flag, so a disabled server is
// omitted rather than written with an invented key.
func TestClaudeWrite_OmitsDisabled(t *testing.T) {
	paths := newTestPaths(t)
	p := &ClaudeProvider{Path: paths.ClaudeJSON}

	require.NoError(t, p.Write(map[string]Server{
		"on":  {Type: TypeStdio, Command: "uvx"},
		"off": {Type: TypeStdio, Command: "npx", Enabled: boolPtr(false)},
	}, nil))

	got, err := p.Read()
	require.NoError(t, err)
	assert.Contains(t, got, "on")
	assert.NotContains(t, got, "off")
}

func TestJSONWrite_LeavesHandConfiguredServersAlone(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CursorMCP, `{
  "mcpServers": {
    "mine": { "type": "stdio", "command": "my-server" }
  }
}`)
	p := &CursorProvider{Path: paths.CursorMCP}

	require.NoError(t, p.Write(map[string]Server{"shared": {Type: TypeStdio, Command: "uvx"}}, nil))

	got, err := p.Read()
	require.NoError(t, err)
	assert.Contains(t, got, "mine", "a server arc does not own must survive")
	assert.Contains(t, got, "shared")
}

func TestJSONWrite_RemovesOwnedServer(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CursorMCP, `{
  "mcpServers": {
    "mine": { "type": "stdio", "command": "my-server" },
    "stale": { "type": "stdio", "command": "uvx" }
  }
}`)
	p := &CursorProvider{Path: paths.CursorMCP}

	require.NoError(t, p.Write(map[string]Server{}, []string{"stale"}))

	got, err := p.Read()
	require.NoError(t, err)
	assert.NotContains(t, got, "stale")
	assert.Contains(t, got, "mine")
}

// A 0-byte mcp.json is what an untouched Cursor install actually has on disk.
func TestJSONRead_EmptyFile(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.CursorMCP, "")
	got, err := (&CursorProvider{Path: paths.CursorMCP}).Read()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestJSONWrite_NoServersDoesNotCreateFile(t *testing.T) {
	paths := newTestPaths(t)
	p := &CursorProvider{Path: paths.CursorMCP}
	require.NoError(t, p.Write(map[string]Server{}, nil))

	_, err := (&CursorProvider{Path: paths.CursorMCP}).Read()
	require.NoError(t, err)
	assert.NoFileExists(t, paths.CursorMCP)
}

func TestBearerEnvVar(t *testing.T) {
	v, ok := bearerEnvVar(map[string]string{"Authorization": "Bearer {env:TOK}"})
	require.True(t, ok)
	assert.Equal(t, "TOK", v)

	_, ok = bearerEnvVar(map[string]string{"authorization": "Bearer {env:TOK}", "X-Other": "1"})
	assert.False(t, ok, "extra headers are not a plain bearer token")

	_, ok = bearerEnvVar(map[string]string{"Authorization": "Bearer literal-token"})
	assert.False(t, ok, "a literal token is not an env reference")
}

func TestJSONObject_RoundTripPreservesOrder(t *testing.T) {
	obj, err := decodeJSONObject([]byte(`{"z":1,"a":{"nested":true},"m":[1,2]}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"z", "a", "m"}, obj.keys)

	obj.set("a", json.RawMessage(`"replaced"`))
	assert.Equal(t, []string{"z", "a", "m"}, obj.keys, "replacing a key keeps its position")

	obj.set("new", json.RawMessage(`1`))
	assert.Equal(t, []string{"z", "a", "m", "new"}, obj.keys)

	obj.delete("m")
	assert.Equal(t, []string{"z", "a", "new"}, obj.keys)

	out, err := obj.encode()
	require.NoError(t, err)
	assert.JSONEq(t, `{"z":1,"a":"replaced","new":1}`, string(out))
}
