---
schema_version: 1
id: d5ffgq8
status: closed
closed: 2026-01-08T00:30:17Z
blocked-by: []
created: 2026-01-08T00:20:45Z
type: task
priority: 2
---
# Add short flags -f and -b to wt delete command

Add short flags to delete command:
- `-f` for `--force`
- `-b` for `--with-branch`

Current flags only have long form. Update flag definitions in DeleteCmd().
