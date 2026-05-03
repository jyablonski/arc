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

## Commands

Promote a draft directory into the canonical store and link it everywhere:

```bash
arc skills add ./my-draft
```

Scaffold a new skill from the built-in template:

```bash
arc skills add --new my-skill
```

Migrate provider-local skills into the canonical store and ensure provider links exist:

```bash
arc skills sync
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
