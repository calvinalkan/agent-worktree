---
schema_version: 1
id: d5fe52r
status: open
blocked-by: []
created: 2026-01-07T22:47:39Z
type: chore
priority: 2
---
# Improve global help output

Add a 2-line description explaining what wt does, and a footer hint about subcommand help.

Changes to run.go printUsage():
- Add description: 'Manages isolated git worktrees with auto-generated names, lifecycle hooks, and metadata tracking. Each worktree gets its own branch and directory.'
- Add footer: 'Run wt <command> --help for more information on a command.'
