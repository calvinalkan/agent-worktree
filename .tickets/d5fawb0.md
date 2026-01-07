---
schema_version: 1
id: d5fawb0
status: open
blocked-by: [d5fattr, d5fav78, d5favh8]
created: 2026-01-07T19:04:12Z
type: task
priority: 1
---
# Implement wt list command

## Overview
Implement the full `wt list` command per SPEC.md.

## Background & Rationale
The list command shows all wt-managed worktrees for the current repository. It supports both human-readable table output and JSON for scripting.

## Current State
list.go has a stub that prints "not implemented yet".

## Implementation Details

### execList Implementation
```go
func execList(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet) error {
    _ = stdin // not used by list command
    jsonOutput, _ := flags.GetBool("json")
    
    // 1. Verify git repository
    repoRoot, err := gitRepoRoot(cfg.EffectiveCwd)
    if err != nil {
        return fmt.Errorf("not a git repository")
    }
    
    // 2. Find worktrees
    baseDir := resolveWorktreeBaseDir(cfg, repoRoot)
    worktrees, err := findWorkstreesWithPaths(fsys, baseDir)
    if err != nil {
        return fmt.Errorf("scanning worktrees: %w", err)
    }
    
    // 3. Output
    if jsonOutput {
        return outputListJSON(stdout, worktrees)
    }
    return outputListTable(stdout, worktrees)
}
```

### WorktreeWithPath for Output
```go
type WorktreeWithPath struct {
    WorktreeInfo
    Path string `json:"path"`
}

func findWorkstreesWithPaths(fsys fs.FS, baseDir string) ([]WorktreeWithPath, error) {
    entries, err := fsys.ReadDir(baseDir)
    if err != nil {
        // Use errors.Is() not os.IsNotExist() per TECH_SPEC
        if errors.Is(err, os.ErrNotExist) {
            return nil, nil
        }
        return nil, err
    }
    
    var result []WorktreeWithPath
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        wtPath := filepath.Join(baseDir, entry.Name())
        info, err := readWorktreeInfo(fsys, wtPath)
        if err != nil {
            // Not a wt-managed worktree, could include with null fields
            // Per SPEC: fields populated from git where possible
            continue
        }
        result = append(result, WorktreeWithPath{
            WorktreeInfo: info,
            Path:         wtPath,
        })
    }
    return result, nil
}
```

### Table Output
```go
func outputListTable(w io.Writer, worktrees []WorktreeWithPath) error {
    if len(worktrees) == 0 {
        return nil // Empty output for no worktrees
    }
    
    // Header
    fprintf(w, "%-15s %-50s %s\n", "NAME", "PATH", "CREATED")
    
    for _, wt := range worktrees {
        age := formatAge(wt.Created)
        fprintf(w, "%-15s %-50s %s\n", wt.Name, wt.Path, age)
    }
    return nil
}

func formatAge(t time.Time) string {
    d := time.Since(t)
    switch {
    case d < time.Minute:
        return "just now"
    case d < time.Hour:
        return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%d hours ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%d days ago", int(d.Hours()/24))
    }
}
```

### JSON Output
```go
func outputListJSON(w io.Writer, worktrees []WorktreeWithPath) error {
    // Build output slice with all fields
    type jsonWorktree struct {
        Name       string     `json:"name"`
        AgentID    *string    `json:"agent_id"`    // nullable
        ID         *int       `json:"id"`          // nullable
        Path       string     `json:"path"`
        BaseBranch string     `json:"base_branch"`
        Created    *time.Time `json:"created"`     // nullable
    }
    
    output := make([]jsonWorktree, len(worktrees))
    for i, wt := range worktrees {
        output[i] = jsonWorktree{
            Name:       wt.Name,
            AgentID:    &wt.AgentID,
            ID:         &wt.ID,
            Path:       wt.Path,
            BaseBranch: wt.BaseBranch,
            Created:    &wt.Created,
        }
    }
    
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(output)
}
```

## Output Formats (per SPEC)

### Default (table)
```
NAME            PATH                                              CREATED
swift-fox       ~/code/worktrees/my-repo/swift-fox                3 days ago
brave-owl       ~/code/worktrees/my-repo/brave-owl                1 hour ago
```

### JSON (--json)
```json
[
  {
    "name": "swift-fox",
    "agent_id": "swift-fox",
    "id": 42,
    "path": "/home/user/code/worktrees/my-repo/swift-fox",
    "base_branch": "main",
    "created": "2025-01-04T10:30:00Z"
  }
]
```

## Edge Cases
- No worktrees: empty output (no error)
- Worktree without .wt/worktree.json: per SPEC, include with null agent_id/id
- Invalid metadata: skip or include with partial info

## Acceptance Criteria
- Lists all wt-managed worktrees for current repo
- Table output with NAME, PATH, CREATED columns
- JSON output with all fields (--json)
- Handles empty worktree list gracefully
- Works with -C flag

## Testing
- List with no worktrees
- List with one worktree
- List with multiple worktrees
- JSON output format
- Table output format
