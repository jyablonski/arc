// Package mcp manages a canonical MCP server list shared across AI coding
// tools (Claude, Codex, Cursor, opencode).
//
// Unlike internal/skills, this cannot be a symlink store: three of the four
// providers keep their servers inside a larger file the tool also owns
// (~/.claude.json holds session state, ~/.codex/config.toml holds the model and
// per-project trust levels, opencode.json holds everything else). Symlinking
// any of those would clobber unrelated config.
//
// So the model is render-and-merge instead: one canonical file, a per-provider
// renderer that translates it into that tool's dialect, and a surgical merge
// that rewrites only the servers arc owns. Ownership is tracked in a state file
// so a server removed from canonical is removed downstream, while servers added
// by hand in a provider are never touched.
package mcp
