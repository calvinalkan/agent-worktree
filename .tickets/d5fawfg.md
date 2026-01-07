---
schema_version: 1
id: d5fawfg
status: open
blocked-by: [d5favt8, d5fav78]
created: 2026-01-07T19:04:30Z
type: task
priority: 2
---
# Implement wt info command

## Overview
Implement the full `wt info` command per SPEC.md.

## Background & Rationale
The info command displays information about the current worktree. It must be run from within a wt-managed worktree. Supports:
- Default key-value output
- JSON output (--json)
- Single field output (--field)

## Current State
info.go will be created by task d5favt8 (info command stub).

## Implementation Details

### execInfo Implementation
```go
func execInfo(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, flags *flag.FlagSet) error {
    _ = stdin // not used by info command
    jsonOutput, _ := flags.GetBool("json")
    field, _ := flags.GetString("field")
    
    // 1. Find worktree root (look for .wt/worktree.json)
    wtPath, err := findWorktreeRoot(fsys, cfg.EffectiveCwd)
    if err != nil {
        return fmt.Errorf("not in a wt-managed worktree")
    }
    
    // 2. Read metadata
    info, err := readWorktreeInfo(fsys, wtPath)
    if err != nil {
        return fmt.Errorf("reading worktree metadata: %w", err)
    }
    
    // 3. Build full info with path
    fullInfo := WorktreeWithPath{
        WorktreeInfo: info,
        Path:         wtPath,
    }
    
    // 4. Output
    if field != "" {
        return outputField(stdout, fullInfo, field)
    }
    if jsonOutput {
        return outputInfoJSON(stdout, fullInfo)
    }
    return outputInfoKeyValue(stdout, fullInfo)
}
```

### Find Worktree Root
```go
// findWorktreeRoot walks up from cwd looking for .wt/worktree.json.
func findWorktreeRoot(fsys fs.FS, startDir string) (string, error) {
    dir := startDir
    for {
        infoPath := filepath.Join(dir, ".wt", "worktree.json")
        if exists, _ := fsys.Exists(infoPath); exists {
            return dir, nil
        }
        
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", errors.New("not in a wt-managed worktree")
        }
        dir = parent
    }
}
```

### Key-Value Output
```go
func outputInfoKeyValue(w io.Writer, info WorktreeWithPath) error {
    fprintf(w, "name:        %s\n", info.Name)
    fprintf(w, "agent_id:    %s\n", info.AgentID)
    fprintf(w, "id:          %d\n", info.ID)
    fprintf(w, "path:        %s\n", info.Path)
    fprintf(w, "base_branch: %s\n", info.BaseBranch)
    fprintf(w, "created:     %s\n", info.Created.Format(time.RFC3339))
    return nil
}
```

### JSON Output
```go
func outputInfoJSON(w io.Writer, info WorktreeWithPath) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(info)
}
```

### Single Field Output
```go
func outputField(w io.Writer, info WorktreeWithPath, field string) error {
    var value string
    switch field {
    case "name":
        value = info.Name
    case "agent_id":
        value = info.AgentID
    case "id":
        value = strconv.Itoa(info.ID)
    case "path":
        value = info.Path
    case "base_branch":
        value = info.BaseBranch
    case "created":
        value = info.Created.Format(time.RFC3339)
    default:
        return fmt.Errorf("unknown field: %s", field)
    }
    fprintln(w, value)
    return nil
}
```

## Output Formats (per SPEC)

### Default (key-value)
```
name:        swift-fox
agent_id:    swift-fox
id:          42
path:        /home/user/code/worktrees/my-repo/swift-fox
base_branch: main
created:     2025-01-04T10:30:00Z
```

### --field id
```
42
```

### --json
```json
{
  "name": "swift-fox",
  "agent_id": "swift-fox",
  "id": 42,
  "path": "/home/user/code/worktrees/my-repo/swift-fox",
  "base_branch": "main",
  "created": "2025-01-04T10:30:00Z"
}
```

## Error Cases
- Not in a wt-managed worktree: exit with error
- .wt/worktree.json missing or invalid: exit with error
- Unknown --field value: exit with error

## Acceptance Criteria
- Shows info when run from wt-managed worktree
- Errors when not in wt-managed worktree
- Key-value output by default
- JSON output with --json
- Single field output with --field
- All fields match SPEC format

## Testing
- Info from worktree root
- Info from subdirectory of worktree
- Info from non-worktree (error)
- --json output
- --field for each field
- --field with invalid field name (error)
