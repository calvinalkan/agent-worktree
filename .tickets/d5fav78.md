---
schema_version: 1
id: d5fav78
status: open
blocked-by: []
created: 2026-01-07T19:01:49Z
type: task
priority: 1
---
# Implement WorktreeInfo struct and metadata read/write

## Overview
Implement the WorktreeInfo struct and functions to read/write .wt/worktree.json metadata files.

## Background & Rationale
Each wt-managed worktree stores metadata in .wt/worktree.json within the worktree directory. This metadata includes:
- name: Directory/branch name
- agent_id: Auto-generated identifier (adjective-animal)
- id: Unique integer for port offsets, container names, etc.
- base_branch: Branch the worktree was created from
- created: ISO 8601 UTC timestamp

This metadata enables:
- Listing worktrees with their creation info
- Providing unique IDs for hooks (port allocation, etc.)
- Tracking lineage (which branch was forked from)

## Current State
No WorktreeInfo struct or metadata functions exist.

## Implementation Details

### Struct Definition (add to run.go)
```go
// WorktreeInfo holds metadata for a wt-managed worktree.
// Stored in .wt/worktree.json within each worktree.
type WorktreeInfo struct {
    Name       string    `json:"name"`
    AgentID    string    `json:"agent_id"`
    ID         int       `json:"id"`
    BaseBranch string    `json:"base_branch"`
    Created    time.Time `json:"created"`
}
```

### Write Function
```go
// writeWorktreeInfo writes metadata to .wt/worktree.json in the worktree.
func writeWorktreeInfo(fsys fs.FS, wtPath string, info WorktreeInfo) error {
    wtDir := filepath.Join(wtPath, ".wt")
    if err := fsys.MkdirAll(wtDir, 0o755); err != nil {
        return fmt.Errorf("creating .wt directory: %w", err)
    }
    
    data, err := json.MarshalIndent(info, "", "  ")
    if err != nil {
        return fmt.Errorf("marshaling worktree info: %w", err)
    }
    
    infoPath := filepath.Join(wtDir, "worktree.json")
    f, err := fsys.Create(infoPath)
    if err != nil {
        return fmt.Errorf("creating worktree.json: %w", err)
    }
    defer f.Close()
    
    if _, err := f.Write(data); err != nil {
        return fmt.Errorf("writing worktree.json: %w", err)
    }
    
    if err := f.Sync(); err != nil {
        return fmt.Errorf("syncing worktree.json: %w", err)
    }
    
    return nil
}
```

### Read Function
```go
// readWorktreeInfo reads metadata from .wt/worktree.json in the worktree.
// Returns os.ErrNotExist if the file doesn't exist.
func readWorktreeInfo(fsys fs.FS, wtPath string) (WorktreeInfo, error) {
    infoPath := filepath.Join(wtPath, ".wt", "worktree.json")
    data, err := fsys.ReadFile(infoPath)
    if err != nil {
        return WorktreeInfo{}, err
    }
    
    var info WorktreeInfo
    if err := json.Unmarshal(data, &info); err != nil {
        return WorktreeInfo{}, fmt.Errorf("parsing worktree.json: %w", err)
    }
    
    return info, nil
}
```

### Find All Worktrees
```go
// findWorktrees scans the given directory for wt-managed worktrees.
// searchDir should be the result of resolveWorktreeBaseDir() - it already
// accounts for whether base path is absolute (includes repo name) or 
// relative (no repo name).
// Returns worktrees that have .wt/worktree.json files.
func findWorktrees(fsys fs.FS, searchDir string) ([]WorktreeInfo, error) {
    entries, err := fsys.ReadDir(searchDir)
    if err != nil {
        // Use errors.Is() not os.IsNotExist() - we can check error values
        // but not call os package functions per TECH_SPEC
        if errors.Is(err, os.ErrNotExist) {
            return nil, nil  // No worktrees yet
        }
        return nil, err
    }
    
    var worktrees []WorktreeInfo
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        wtPath := filepath.Join(searchDir, entry.Name())
        info, err := readWorktreeInfo(fsys, wtPath)
        if err != nil {
            continue  // Skip non-wt directories
        }
        worktrees = append(worktrees, info)
    }
    
    return worktrees, nil
}
```

**Note**: The `searchDir` parameter should come from `resolveWorktreeBaseDir(cfg, repoRoot)` 
which handles the absolute vs relative path logic (see task d5favh8).

## Acceptance Criteria
- WorktreeInfo struct matches SPEC.md schema
- writeWorktreeInfo creates .wt/ directory and writes JSON
- readWorktreeInfo reads and parses JSON
- findWorktrees scans base directory for managed worktrees
- All functions use fs.FS abstraction (not os package directly)

## Testing
- Test round-trip write/read
- Test findWorktrees with mixed directories
