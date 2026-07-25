package mcp

import (
	"os"
	"path/filepath"
)

type Paths struct {
	CanonicalFile  string // default: ~/ai/mcp.json
	StateFile      string // default: ~/.config/arc/mcp-state.json
	ClaudeJSON     string // default: ~/.claude.json
	CodexConfig    string // default: ~/.codex/config.toml
	CursorMCP      string // default: ~/.cursor/mcp.json
	OpencodeConfig string // default: ~/.config/opencode/opencode.json

	// ClaudeAuthCache is where Claude Code records remote MCP servers whose
	// OAuth needs redoing. Read-only; arc never writes it.
	ClaudeAuthCache string // default: ~/.claude/mcp-needs-auth-cache.json
}

// DefaultPaths reads HOME and honors env overrides:
//
//	ARC_MCP_FILE          -> canonical mcp.json
//	ARC_MCP_STATE         -> ownership state file
//	ARC_CLAUDE_JSON       -> ~/.claude.json
//	ARC_CODEX_CONFIG      -> ~/.codex/config.toml
//	ARC_CURSOR_MCP        -> ~/.cursor/mcp.json
//	ARC_OPENCODE_CONFIG   -> ~/.config/opencode/opencode.json
//	ARC_CLAUDE_DIR        -> ~/.claude (for the MCP auth cache)
//
// The canonical file defaults next to the shared skills store so ~/ai holds the
// whole shared-config set (skills/, AGENTS.md, mcp.json); ARC_SKILLS_ROOT moves
// it along with the rest.
func DefaultPaths() Paths {
	home := os.Getenv("HOME")

	canonical := os.Getenv("ARC_MCP_FILE")
	if canonical == "" {
		aiRoot := filepath.Join(home, "ai")
		if root := os.Getenv("ARC_SKILLS_ROOT"); root != "" {
			aiRoot = filepath.Dir(root)
		}
		canonical = filepath.Join(aiRoot, "mcp.json")
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	claudeDir := envOr("ARC_CLAUDE_DIR", filepath.Join(home, ".claude"))

	return Paths{
		CanonicalFile:  canonical,
		StateFile:      envOr("ARC_MCP_STATE", filepath.Join(configHome, "arc", "mcp-state.json")),
		ClaudeJSON:     envOr("ARC_CLAUDE_JSON", filepath.Join(home, ".claude.json")),
		CodexConfig:    envOr("ARC_CODEX_CONFIG", filepath.Join(home, ".codex", "config.toml")),
		CursorMCP:      envOr("ARC_CURSOR_MCP", filepath.Join(home, ".cursor", "mcp.json")),
		OpencodeConfig: envOr("ARC_OPENCODE_CONFIG", filepath.Join(configHome, "opencode", "opencode.json")),

		ClaudeAuthCache: filepath.Join(claudeDir, "mcp-needs-auth-cache.json"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
