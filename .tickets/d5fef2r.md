---
schema_version: 1
id: d5fef2r
status: closed
closed: 2026-01-07T23:11:10Z
blocked-by: []
created: 2026-01-07T23:08:59Z
type: chore
priority: 2
---
# Move Go code into cmd/wt/ with cmd_ prefix for commands

Reorganize Go files to separate commands from library helpers.

Create cmd/wt/ directory and move all Go files there. Command files get cmd_ prefix.

## File moves:

| From | To |
|------|-----|
| main.go | cmd/wt/main.go |
| run.go | cmd/wt/cmd_run.go |
| command.go | cmd/wt/command.go |
| create.go | cmd/wt/cmd_create.go |
| delete.go | cmd/wt/cmd_delete.go |
| list.go | cmd/wt/cmd_list.go |
| info.go | cmd/wt/cmd_info.go |
| git.go | cmd/wt/git.go |
| hooks.go | cmd/wt/hooks.go |
| names.go | cmd/wt/names.go |
| *_test.go | cmd/wt/*_test.go (matching names) |

## Other changes:

- Update Makefile: build path changes to ./cmd/wt
- No code changes needed (still package main)
- No go.mod changes needed
