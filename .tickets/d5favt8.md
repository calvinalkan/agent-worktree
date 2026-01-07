---
schema_version: 1
id: d5favt8
status: open
blocked-by: []
created: 2026-01-07T19:03:05Z
type: task
priority: 2
---
# Implement info command stub

## Overview
Add the info command stub (matching existing create/list/delete pattern).

## Background & Rationale
The SPEC.md defines four commands: create, list, info, delete. The scaffolding has stubs for create, list, delete but info is missing entirely. This task adds the stub so all commands appear in help.

## Current State
- create.go, list.go, delete.go exist with stubs
- No info.go file
- Run() only registers create, list, delete commands

## Implementation Details

### Create info.go
```go
package main

import (
    "context"
    "io"

    "github.com/calvinalkan/agent-task/pkg/fs"
    flag "github.com/spf13/pflag"
)

// InfoCmd returns the info command.
func InfoCmd(cfg Config, fsys fs.FS) *Command {
    flags := flag.NewFlagSet("info", flag.ContinueOnError)
    flags.BoolP("help", "h", false, "Show help")
    flags.Bool("json", false, "Output as JSON")
    flags.String("field", "", "Output only the specified field value")

    return &Command{
        Flags: flags,
        Usage: "info [flags]",
        Short: "Show current worktree info",
        Long: `Display information about the current worktree.

Must be run from within a wt-managed worktree.`,
        Exec: func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args []string) error {
            return execInfo(ctx, stdin, stdout, stderr, cfg, fsys, flags)
        },
    }
}

func execInfo(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet) error {
    jsonOutput, _ := flags.GetBool("json")
    field, _ := flags.GetString("field")

    // TODO: Implement
    _ = stdin  // not used by info command
    _ = jsonOutput
    _ = field
    _ = fsys

    fprintln(stdout, "info: not implemented yet")
    return nil
}
```

### Update run.go
Add InfoCmd to the commands slice:
```go
commands := []*Command{
    CreateCmd(cfg, fsys),
    ListCmd(cfg, fsys),
    InfoCmd(cfg, fsys),   // ADD THIS
    DeleteCmd(cfg, fsys),
}
```

## Acceptance Criteria
- `wt info --help` shows usage
- `wt --help` lists info command
- info command returns "not implemented yet" for now

## Testing
- Add test for info --help in run_test.go
