---
schema_version: 1
id: d5fezsr
status: open
blocked-by: []
created: 2026-01-07T23:44:39Z
type: chore
priority: 2
---
# Improve version output when built without ldflags

Current version output when built without ldflags:

  wt dev (none, unknown)

This looks odd. When commit/date aren't set, show cleaner output:

  wt dev (built from source)

Change in cmd/wt/cmd_run.go:

```go
if *flagVersion {
    if commit == "none" && date == "unknown" {
        fprintf(stdout, "wt %s (built from source)\n", version)
    } else {
        fprintf(stdout, "wt %s (%s, %s)\n", version, commit, date)
    }
    return 0
}
```
