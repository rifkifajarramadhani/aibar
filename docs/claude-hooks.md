# Claude Code refresh hooks

aibar watches Claude Code project JSONL files, but hooks can reduce the delay
after a session finishes. Hook commands are intentionally opt-in; aibar never
edits `~/.claude/settings.json` automatically.

Add the following entries to the existing `hooks` object in your Claude Code
settings. Merge them with existing hooks instead of replacing the whole
object:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh -c 'aibar refresh >/dev/null 2>&1 || true'"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh -c 'aibar refresh >/dev/null 2>&1 || true'"
          }
        ]
      }
    ]
  }
}
```

If Claude Code does not inherit the directory containing aibar, use its
absolute path, for example:

```sh
sh -c '/home/overhaul/.local/bin/aibar refresh >/dev/null 2>&1 || true'
```

The command is best-effort so a stopped aibar daemon never blocks or fails a
Claude Code session. The daemon still enforces Claude's five-minute minimum
network interval, so repeated hook calls are coalesced safely.
