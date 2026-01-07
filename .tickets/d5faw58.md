---
schema_version: 1
id: d5faw58
status: closed
closed: 2026-01-07T20:54:09Z
blocked-by: [d5fattr, d5fav78, d5favcr, d5favh8, d5favpr]
created: 2026-01-07T19:03:49Z
type: task
priority: 1
---
# Implement wt create command

## Overview
Implement the full `wt create` command per SPEC.md.

## Background & Rationale
The create command is the primary entry point for users. It:
1. Generates a unique agent_id and ID
2. Creates a git worktree with a new branch
3. Writes metadata to .wt/worktree.json
4. Optionally copies uncommitted changes (--with-changes)
5. Runs post-create hook (with rollback on failure)

## Current State
create.go has a stub that prints "not implemented yet".

## Implementation Details

### Command Flow
```
1. Verify in git repository (gitRepoRoot)
2. Resolve base branch (--from-branch or current branch)
3. Find existing worktrees to get:
   - Next ID (max existing ID + 1, starting at 1)
   - Existing names for collision detection
4. Generate agent_id (unique adjective-animal)
5. Set name to --name flag value or agent_id
6. Resolve worktree path
7. Create base directory if needed
8. git worktree add -b <name> <path> <base-branch>
9. Write .wt/worktree.json metadata
10. If --with-changes: copy uncommitted changes
11. Run post-create hook
12. If hook fails: rollback (remove worktree + delete branch)
13. Print success output
```

### execCreate Implementation
```go
func execCreate(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, env map[string]string, flags *flag.FlagSet) error {
    _ = stdin // not used by create command
    customName, _ := flags.GetString("name")
    fromBranch, _ := flags.GetString("from-branch")
    withChanges, _ := flags.GetBool("with-changes")
    
    // 1. Verify git repository
    repoRoot, err := gitRepoRoot(cfg.EffectiveCwd)
    if err != nil {
        return fmt.Errorf("not a git repository")
    }
    
    // 2. Resolve base branch
    baseBranch := fromBranch
    if baseBranch == "" {
        baseBranch, err = gitCurrentBranch(cfg.EffectiveCwd)
        if err != nil {
            return fmt.Errorf("cannot determine current branch: %w", err)
        }
    }
    
    // 3. Find existing worktrees
    // Note: resolveWorktreeBaseDir already handles repo name for absolute paths
    baseDir := resolveWorktreeBaseDir(cfg, repoRoot)
    existing, err := findWorktrees(fsys, baseDir)
    if err != nil {
        return fmt.Errorf("scanning existing worktrees: %w", err)
    }
    
    // Calculate next ID
    nextID := 1
    for _, wt := range existing {
        if wt.ID >= nextID {
            nextID = wt.ID + 1
        }
    }
    
    // 4. Generate agent_id
    existingNames := getExistingNames(existing)
    agentID, err := generateAgentID(existingNames)
    if err != nil {
        return err
    }
    
    // 5. Set name
    name := customName
    if name == "" {
        name = agentID
    }
    
    // Check name collision
    for _, n := range existingNames {
        if n == name {
            return fmt.Errorf("name %q already in use", name)
        }
    }
    
    // 6-13: See full description in ticket
    ...
}
```

## Output Format (per SPEC)
```
Created worktree:
  name:        swift-fox
  agent_id:    swift-fox
  id:          42
  path:        ~/code/worktrees/my-repo/swift-fox
  branch:      swift-fox
  from:        main
```

## Implementation Notes

### Flag Name Fix
The existing create.go stub uses `--copy-changes` but SPEC.md says `--with-changes`. 
Fix the flag name to match SPEC.

### --with-changes Implementation
The --with-changes flag implementation details are in task d5faxh0. This task implements
the basic create flow; --with-changes can initially be a TODO that's completed by d5faxh0.

### Environment Passing
HookRunner needs the env map (see d5favpr). The command needs to receive env from Run()
and pass it through.

### Concurrent Safety
For production robustness, see d5faxn8 for file locking to prevent ID collisions.

## Acceptance Criteria
- Creates worktree with git worktree add
- Generates unique agent_id
- Assigns incrementing ID  
- Creates .wt/worktree.json metadata
- Respects --name flag
- Respects --from-branch flag
- Runs post-create hook (with env passed through)
- Rolls back on hook failure
- Prints formatted output
- Flag name is --with-changes (not --copy-changes)

Note: Full --with-changes implementation is in task d5faxh0.

## Testing
- Create worktree with defaults
- Create with custom name
- Create from specific branch
- Hook success and failure cases
