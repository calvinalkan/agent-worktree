---
schema_version: 1
id: d5fayh0
status: closed
closed: 2026-01-07T21:49:55Z
blocked-by: []
created: 2026-01-07T19:08:52Z
type: task
priority: 2
---
# Add helper function to extract worktree path from create output

## Overview
Add a helper function to extract the worktree path from `wt create` output for use in tests.

## Background & Rationale
Multiple tests need to get the worktree path from create output to then run commands from that directory or verify files exist there.

## Implementation

```go
// extractPath extracts the path from wt create output.
// Output format is:
//   Created worktree:
//     name:        swift-fox
//     ...
//     path:        /path/to/worktree
//     ...
func extractPath(createOutput string) string {
    for _, line := range strings.Split(createOutput, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "path:") {
            return strings.TrimSpace(strings.TrimPrefix(line, "path:"))
        }
    }
    return ""
}

// extractField extracts any field from wt create/info output.
func extractField(output, field string) string {
    prefix := field + ":"
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, prefix) {
            return strings.TrimSpace(strings.TrimPrefix(line, prefix))
        }
    }
    return ""
}
```

## Location
Add to testing_test.go

## Acceptance Criteria
- extractPath correctly parses create output
- extractField works for any field (name, id, agent_id, etc.)
- Returns empty string if field not found
