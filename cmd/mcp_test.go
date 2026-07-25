package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
)

func setupMCPEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("ARC_MCP_FILE", filepath.Join(root, "ai", "mcp.json"))
	t.Setenv("ARC_MCP_STATE", filepath.Join(root, "state", "mcp-state.json"))
	t.Setenv("ARC_CLAUDE_JSON", filepath.Join(root, "claude.json"))
	t.Setenv("ARC_CODEX_CONFIG", filepath.Join(root, "codex", "config.toml"))
	t.Setenv("ARC_CURSOR_MCP", filepath.Join(root, "cursor", "mcp.json"))
	t.Setenv("ARC_OPENCODE_CONFIG", filepath.Join(root, "opencode", "opencode.json"))
	t.Setenv("ARC_CLAUDE_DIR", filepath.Join(root, "dotclaude"))
	return root
}

func resetMCPFlags() {
	mcpDryRun = false
	mcpSyncForce = false
	mcpAddForce = false
	mcpProvider = ""
	mcpAddType = ""
	mcpAddCommand = ""
	mcpAddArgs = nil
	mcpAddEnv = nil
	mcpAddURL = ""
	mcpAddHeaders = nil
	mcpAddDisabled = false
	mcpAddProviders = nil
}

// --provider is a persistent flag on `arc mcp`, so it has to constrain add and
// remove too. Wiring these through the unfiltered manager meant the flag was
// accepted and silently ignored, writing to every provider.
func TestMCPAddCmd_HonorsProviderFilter(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpProvider = "claude"
	mcpAddCommand = "uvx"
	mcpAddArgs = []string{"context7-mcp"}
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"ctx7"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "claude.json")); err != nil {
		t.Errorf("claude should have been written: %v", err)
	}
	for _, name := range []string{"codex/config.toml", "cursor/mcp.json", "opencode/opencode.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("%s was written despite --provider claude", name)
		}
	}
}

func TestMCPRemoveCmd_HonorsProviderFilter(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpAddCommand = "uvx"
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"ctx7"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	codex := filepath.Join(root, "codex", "config.toml")
	if _, err := os.Stat(codex); err != nil {
		t.Fatalf("precondition: codex not written: %v", err)
	}

	// Remove scoped to claude only: codex keeps its copy.
	mcpProvider = "claude"
	if err := mcpRemoveCmd.RunE(mcpRemoveCmd, []string{"ctx7"}); err != nil {
		t.Fatalf("remove: %v", err)
	}

	data, err := os.ReadFile(codex)
	if err != nil {
		t.Fatalf("read codex: %v", err)
	}
	if !strings.Contains(string(data), "ctx7") {
		t.Error("codex copy was removed despite --provider claude")
	}
}

func TestMCPCmd_RejectsUnknownProvider(t *testing.T) {
	setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpProvider = "nope"
	if err := mcpListCmd.RunE(mcpListCmd, nil); err == nil {
		t.Error("expected an error for an unknown provider")
	}
}

func TestMCPAddCmd_RequiresExactlyOneTransport(t *testing.T) {
	setupMCPEnv(t)
	defer resetMCPFlags()

	resetMCPFlags()
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"srv"}); err == nil {
		t.Error("expected an error when neither --command nor --url is given")
	}

	resetMCPFlags()
	mcpAddCommand = "uvx"
	mcpAddURL = "https://example.com/mcp"
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"srv"}); err == nil {
		t.Error("expected an error when both --command and --url are given")
	}
}

func TestMCPAddCmd_BuildsServerFromFlags(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpAddURL = "http://mcp.home/mcp"
	mcpAddHeaders = []string{"Authorization=Bearer {env:HOMELAB_MCP_TOKEN}"}
	mcpAddProviders = []string{"claude", "cursor"}
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"homelab"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "ai", "mcp.json"))
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	var f struct {
		MCPServers map[string]struct {
			Type      string            `json:"type"`
			URL       string            `json:"url"`
			Headers   map[string]string `json:"headers"`
			Providers []string          `json:"providers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse canonical: %v", err)
	}
	got := f.MCPServers["homelab"]
	if got.Type != "http" {
		t.Errorf("type inferred from --url should be http, got %q", got.Type)
	}
	if got.Headers["Authorization"] != "Bearer {env:HOMELAB_MCP_TOKEN}" {
		t.Errorf("header not parsed: %#v", got.Headers)
	}
	if len(got.Providers) != 2 {
		t.Errorf("restrict-to not recorded: %#v", got.Providers)
	}
}

func TestMCPAddCmd_RejectsMalformedKeyValueFlag(t *testing.T) {
	setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpAddCommand = "uvx"
	mcpAddEnv = []string{"NOEQUALS"}
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"srv"}); err == nil {
		t.Error("expected an error for an --env value without '='")
	}
}

func TestMCPAddCmd_RejectsInlineCredential(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpAddURL = "https://example.com/mcp"
	mcpAddHeaders = []string{"Authorization=Bearer ghp_literaltoken12345"}
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"leaky"}); err == nil {
		t.Error("expected an inline credential to be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "mcp.json")); err == nil {
		t.Error("canonical must not be created when validation fails")
	}
}

func TestMCPAddForce_DoesNotForceProviderConflicts(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	canonical := filepath.Join(root, "ai", "mcp.json")
	requireDir := filepath.Dir(canonical)
	if err := os.MkdirAll(requireDir, filemode.Dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, canonical, `{
  "mcpServers": {
    "shared": {"type": "stdio", "command": "old-canonical"}
  }
}`)
	cursor := filepath.Join(root, "cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursor), filemode.Dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, cursor, `{
  "mcpServers": {
    "shared": {"type": "stdio", "command": "hand-managed"}
  }
}`)

	mcpAddForce = true
	mcpAddCommand = "new-canonical"
	err := mcpAddCmd.RunE(mcpAddCmd, []string{"shared"})
	if !errors.Is(err, ErrMCPConflict) {
		t.Fatalf("add --force should still report the provider conflict, got %v", err)
	}
	data, readErr := os.ReadFile(cursor)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := string(data); !strings.Contains(got, "hand-managed") {
		t.Fatalf("provider conflict was overwritten: %s", got)
	}
}

func TestMCPDryRun_WritesNothing(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	mcpDryRun = true
	mcpAddCommand = "uvx"
	if err := mcpAddCmd.RunE(mcpAddCmd, []string{"ctx7"}); err != nil {
		t.Fatalf("add --dry-run: %v", err)
	}
	for _, name := range []string{"ai/mcp.json", "claude.json", "codex/config.toml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("--dry-run created %s", name)
		}
	}
}

func TestMCPValidateCmd_CleanCanonical(t *testing.T) {
	root := setupMCPEnv(t)
	defer resetMCPFlags()
	resetMCPFlags()

	path := filepath.Join(root, "ai", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), filemode.Dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"mcpServers":{"ctx7":{"type":"stdio","command":"uvx"}}}`)

	if err := mcpValidateCmd.RunE(mcpValidateCmd, nil); err != nil {
		t.Errorf("validate: %v", err)
	}
}
