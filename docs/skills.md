# Shared AI skills

`arc skills` keeps one canonical skill directory and links each skill into the supported AI tools. The canonical store is `~/ai/skills/`.

Provider-local entries that contain real files are left alone. `arc` only creates, updates, or removes the symlinks and metadata that it owns.

## Skill layout

Each skill has its own directory and a `SKILL.md` file with YAML frontmatter:

```text
~/ai/skills/
└── my-skill/
    └── SKILL.md
```

The frontmatter `name` should match the directory name. Run `arc skills validate` to check the schema. Use `--fix` to rename a canonical directory when its name does not match the frontmatter.

## Provider behavior

Skills are linked into the provider directories used by Claude, Codex, Cursor, and opencode. Codex also uses an `agents/openai.yaml` file for invocation policy.

When a skill sets `disable-model-invocation: true`, `arc skills sync` writes the equivalent Codex policy:

```yaml
policy:
  allow_implicit_invocation: false
```

This prevents automatic invocation while preserving explicit `$skill-name` invocation. Existing OpenAI metadata is preserved. Claude reads `disable-model-invocation` and `user-invocable` directly.

## Explicit-only skills

Create a skill with both `disable-model-invocation: true` and `user-invocable: true` when it should run only after an explicit request:

```markdown
---
name: manual-review
description: Review the current changes only when explicitly invoked by the user.
disable-model-invocation: true
user-invocable: true
---

# Manual review

Inspect the current changes and report correctness issues.
```

Preview the result, then apply it:

```bash
arc skills sync --dry-run
arc skills sync
```

For background on the Codex metadata translation, see [openai/codex#10585](https://github.com/openai/codex/issues/10585).

## Commands

| Command | Purpose |
| --- | --- |
| `arc skills add ./draft` | Promote a draft skill directory into the canonical store. |
| `arc skills add --new my-skill` | Create a new skill from the built-in template. |
| `arc skills sync` | Link canonical skills into each provider. Use `--dry-run` to preview. |
| `arc skills export ./backup` | Copy canonical skills into another parent directory. |
| `arc skills list` | Show canonical skills and provider status. |
| `arc skills validate --fix` | Validate frontmatter and fix supported naming mismatches. |
| `arc skills remove my-skill` | Remove a canonical skill and its provider symlinks. Real provider files are preserved. |
| `arc skills prune` | Remove dangling provider symlinks. |

Sync is one-way. `~/ai/skills/` is the source of truth, and provider-local real content is left for manual review.
