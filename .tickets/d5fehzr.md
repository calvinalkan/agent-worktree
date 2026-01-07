---
schema_version: 1
id: d5fehzr
status: closed
closed: 2026-01-07T23:16:19Z
blocked-by: []
created: 2026-01-07T23:15:11Z
type: chore
priority: 2
---
# Improve signal handling messages

Make signal handling messages clearer and more helpful.

Current messages are confusing ('graceful shutdown ok (130)').

New messages:

First signal:
  'Interrupted, waiting up to 5s for cleanup... (Ctrl+C again to force exit)'

Cleanup complete:
  'Cleanup complete.'

Timeout:
  'Cleanup timed out, forced exit.'

Second signal:
  'Forced exit.'

Changes in cmd/wt/cmd_run.go signal handling section.
