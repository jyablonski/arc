# Shared AI Skills

`arc skills` manages a canonical `~/ai/skills/<name>/SKILL.md` store and symlinks each skill into every supported AI provider directory.

Supported provider targets include Claude, Codex, Cursor, and opencode. Provider slots that already hold real content are left alone; `arc` only creates or removes the symlinks it owns.

## Skill Format

Each skill lives in its own directory and must include a `SKILL.md` file with YAML frontmatter.

```text
~/ai/skills/
└── my-skill/
    └── SKILL.md
```

Validation is strict. The frontmatter name should match the canonical directory name, and `arc skills validate --fix` can rename directories when the metadata and directory drift.

Claude respects the `disable-model-invocation` and `user-invocable` frontmatter fields directly. Codex requires a separate `agents/openai.yaml` file, so `arc skills sync` translates an explicit `disable-model-invocation` value into the native Codex policy. `disable-model-invocation: true` writes `policy.allow_implicit_invocation: false`, preventing automatic model invocation while preserving explicit `$skill-name` invocation. Existing OpenAI metadata such as interface settings and dependencies is preserved.

This translation is necessary because Codex does not currently use the `disable-model-invocation` frontmatter field directly. See [openai/codex#10585](https://github.com/openai/codex/issues/10585) for background.

### Disable Implicit Model Invocation

For a skill that should run only when explicitly invoked, create `~/ai/skills/manual-review/SKILL.md` with both `disable-model-invocation: true` and `user-invocable: true`:

```markdown
---
name: manual-review
description: Review the current changes only when explicitly invoked by the user.
disable-model-invocation: true
user-invocable: true
---

# Manual Review

Inspect the current changes and report correctness issues.
```

Preview the sync, then apply it:

```bash
arc skills sync --dry-run
arc skills sync
```

Sync links the skill into each provider and creates `~/ai/skills/manual-review/agents/openai.yaml` for Codex:

```yaml
policy:
  allow_implicit_invocation: false
```

Codex will not select this skill automatically, but the user can still invoke it explicitly with `$manual-review`.

## Commands

Promote a draft directory into the canonical store and link it everywhere:

```bash
arc skills add ./my-draft
```

Scaffold a new skill from the built-in template:

```bash
arc skills add --new my-skill
```

Ensure every canonical skill is linked into each provider. Sync is one-way: `~/ai/skills` is the source of truth, and provider-local real content is left alone for manual review.

```bash
arc skills sync
```

Copy all canonical skills into another parent directory:

```bash
arc skills export ./skills-backup
```

Show canonical skills and per-provider symlink status:

```bash
arc skills list
```

Validate skill frontmatter and fix supported naming issues:

```bash
arc skills validate --fix
```

Remove a canonical skill and sweep its provider symlinks. Provider-local real content is not removed:

```bash
arc skills remove my-skill
```

Remove dangling provider symlinks:

```bash
arc skills prune
```
