# Local usage statistics

`arc` records a small local usage record for most commands so `arc stats` can show which workflows you use and how long they take.

Each record contains only:

- the command path, such as `update system`
- whether it succeeded
- how long it ran
- the timestamp

Arguments, flag values, environment variables, credentials, and command output are not recorded. Nothing is uploaded.

## View statistics

```bash
arc stats
arc stats --json
```

The log is stored at:

- Linux: `$XDG_STATE_HOME/arc/invocations.jsonl`, or `~/.local/state/arc/invocations.jsonl` by default
- macOS: `~/Library/Application Support/arc/invocations.jsonl`

## Disable tracking

Set `ARC_NO_TRACK=1` in the environment before running `arc`:

```bash
ARC_NO_TRACK=1 arc update system
```

To disable it for every shell session, export the variable in your shell profile. `arc stats` does not record itself.
