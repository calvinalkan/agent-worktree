---
schema_version: 1
id: d5fatn8
status: open
blocked-by: []
created: 2026-01-07T19:00:37Z
type: task
priority: 2
---
# Add --version global flag

## Overview
Add the --version/-v global flag to display version information per SPEC.md.

## Background & Rationale
The SPEC mandates a --version flag in the Global Flags table. The scaffolding already has version/commit/date constants in main.go but the flag isn't wired up yet.

## Current State
- main.go has: version = "dev", commit = "none", date = "unknown"
- run.go parses -h/--help, -C/--cwd, -c/--config but NOT --version

## Implementation Details
1. Add flagVersion to global flags in Run():
   ```go
   flagVersion := globalFlags.BoolP("version", "v", false, "Show version and exit")
   ```

2. After parsing, check flagVersion before loading config:
   ```go
   if *flagVersion {
       fprintln(stdout, "wt", version, commit, date)
       return 0
   }
   ```

3. Update globalOptionsHelp string to include -v/--version

## Acceptance Criteria
- `wt --version` prints version info and exits 0
- `wt -v` prints version info and exits 0
- Version info appears in global help output

## Testing
- Add test cases in run_test.go for --version and -v flags
