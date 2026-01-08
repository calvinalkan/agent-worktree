---
schema_version: 1
id: d5ffsb8
status: open
blocked-by: []
created: 2026-01-08T00:39:09Z
type: feature
priority: 3
---
# Print 'error:' prefix in red when running in TTY

When running interactively in a terminal (TTY), the 'error:' prefix should be displayed in red for better visibility.

**Current output:**
```
error: unknown command: ls
```

**Expected output (in TTY):**
```
\033[31merror:\033[0m unknown command: ls
```

Only apply color when stdout/stderr is a TTY. Non-interactive usage (pipes, scripts) should remain uncolored.

Handle this at the top-level error output, not in individual commands.

**Note:** There is already an `IsTerminal()` function available for TTY detection.
