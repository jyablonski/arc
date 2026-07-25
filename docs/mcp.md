# Shared MCP Servers

`arc mcp` keeps one canonical list of MCP servers in sync across Claude, Codex, Cursor, and opencode.

The canonical file is:

```text
~/ai/mcp.json
```

It sits alongside `~/ai/skills/` and `~/ai/AGENTS.md`, so the whole shared AI-config set lives in one directory.

## How it differs from `skills` and `rules`

`arc skills` and `arc rules` are symlink stores. `arc mcp` cannot be, because three of the four providers keep their servers inside a larger file the tool also owns:

| Provider | File                                       | What else is in it                                    |
| -------- | ------------------------------------------ | ----------------------------------------------------- |
| Claude   | `~/.claude.json` (`mcpServers`)            | session state, onboarding flags, per-project settings |
| Codex    | `~/.codex/config.toml` (`[mcp_servers.*]`) | model settings, per-project trust levels, plugins     |
| Cursor   | `~/.cursor/mcp.json` (`mcpServers`)        | nothing else                                          |
| opencode | `~/.config/opencode/opencode.json` (`mcp`) | theme, model, everything else                         |

Symlinking any of those would clobber unrelated config. So `arc` renders the canonical list into each provider's dialect and merges it in surgically, rewriting only the servers it wrote itself.

Ownership is tracked in `~/.config/arc/mcp-state.json`. That is what makes the merge safe in both directions: a server dropped from canonical is removed downstream because `arc` knows it put it there, and a server added by hand in Cursor is left alone because `arc` knows it did not.

## Canonical format

The canonical dialect is Claude's and Cursor's shape — those two need no translation, and it is the most expressive of the four.

```json
{
  "mcpServers": {
    "context7": {
      "type": "stdio",
      "command": "uvx",
      "args": ["context7-mcp@latest"],
      "env": { "FASTMCP_LOG_LEVEL": "ERROR" }
    },
    "homelab": {
      "type": "http",
      "url": "http://mcp.home/mcp",
      "headers": { "Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}" }
    },
    "figma": {
      "type": "sse",
      "url": "http://127.0.0.1:3845/mcp",
      "enabled": false,
      "providers": ["claude", "cursor"]
    }
  }
}
```

- `type` is `stdio`, `http`, or `sse`. It is inferred from `command` or `url` when omitted.
- `enabled: false` keeps a server in canonical without activating it.
- `providers` restricts a server to a subset of tools. Omit it to sync everywhere.

## Credentials

Never put a literal token in `~/ai/mcp.json` — it is a file you will want to keep in a dotfile repo. Reference the environment instead:

```json
"headers": { "Authorization": "Bearer {env:HOMELAB_MCP_TOKEN}" }
```

`arc` translates `{env:VAR}` into whatever each provider supports, and refuses to write or import a server carrying an inline credential. `arc ai health` warns when a referenced variable is not actually set in your shell.

## Translation is lossy

The four tools do not agree on much, so some servers cannot reach every provider. `arc` reports what it skipped instead of writing config that would silently not work.

| Concept | Claude / Cursor          | Codex                         | opencode                                                    |
| ------- | ------------------------ | ----------------------------- | ----------------------------------------------------------- |
| Format  | JSON, `mcpServers`       | TOML, `[mcp_servers.*]`       | JSON, `mcp`                                                 |
| stdio   | `command`, `args`, `env` | `command`, `args`, `env`      | `type: "local"`, `command` as one argv array, `environment` |
| HTTP    | `url`, `headers`         | `url`, static or env-backed headers | `type: "remote"`, `url`, `headers`                          |
| SSE     | `type: "sse"`            | **unsupported**               | folded into `remote`                                        |
| Disable | omitted from the file    | `enabled = false`             | `enabled = false`                                           |

Two consequences worth knowing:

- **Codex has no SSE transport.** Static headers, exact environment-backed headers, and bearer tokens from the environment are translated into Codex's native fields; header values that mix literals with environment references are reported as unsupported.
- **Claude and Cursor have no file-level disable flag.** A disabled server is omitted from those two files rather than written with an invented key.

## Commands

Seed canonical from whatever is already configured in your tools. This is the migration path onto a shared store — without it, adopting one means retyping every server by hand:

```bash
arc mcp import
```

Import never overwrites canonical. A name that already exists is skipped when identical and reported as a conflict when it differs. Nothing is pushed back out, so follow it with a sync.

Render canonical into every provider:

```bash
arc mcp sync
```

Sync is one-way: `~/ai/mcp.json` is the source of truth. Servers `arc` previously wrote are updated or removed to match; servers it did not write are reported as conflicts and left untouched. Use `--force` to overwrite them anyway, and `--dry-run` to see the plan first.

Show every canonical server with one column per provider:

```bash
arc mcp list
```

| Status        | Meaning                                                        |
| ------------- | -------------------------------------------------------------- |
| `ok`          | present and matching canonical                                 |
| `missing`     | belongs here but has not been written yet                      |
| `drift`       | `arc` owns it and it was edited elsewhere; sync will overwrite |
| `conflict`    | configured by hand and differs; sync leaves it alone           |
| `unsupported` | this provider's dialect cannot express it                      |
| `disabled`    | disabled in canonical and correctly absent                     |
| `excluded`    | restricted to other providers                                  |

Add a server and sync it out in one step:

```bash
arc mcp add context7 --command uvx --arg context7-mcp@latest
arc mcp add homelab --url http://mcp.home/mcp \
  --header 'Authorization=Bearer {env:HOMELAB_MCP_TOKEN}'
```

Remove a server from canonical and sweep it from every provider `arc` wrote it to:

```bash
arc mcp remove context7
```

Check the schema, each provider's dialect, and referenced environment variables:

```bash
arc mcp validate
```

Every command takes `--json` and `--provider` (a comma-separated subset of `claude,codex,cursor,opencode`).

## Health

`arc ai health` includes MCP checks when a canonical file exists:

- `mcp` — how many provider slots are missing, drifted, or conflicting
- `mcp env` — referenced `{env:VAR}` variables that are not set in the current shell
- `mcp auth` — remote servers Claude Code recorded as needing re-authentication, read from `~/.claude/mcp-needs-auth-cache.json`

These are warnings, not failures. A machine with no canonical MCP file is a normal state and reports nothing.
