---
schema_version: 1
id: d5fawn0
status: closed
closed: 2026-01-07T21:09:49Z
blocked-by: [d5fattr, d5fav78, d5favh8, d5favpr]
created: 2026-01-07T19:04:52Z
type: task
priority: 1
---
# Implement wt delete command

## Overview
Implement the full `wt delete` command per SPEC.md.

## Background & Rationale
The delete command removes a worktree by name. It:
1. Runs pre-delete hook (abort on failure)
2. Removes the git worktree
3. Optionally deletes the branch
4. Prunes worktree metadata

## Current State
- delete.go has a stub that prints "not implemented yet"
- stdin is now wired through: `execDelete(ctx, stdin, stdout, stderr, cfg, fsys, flags, args)`

## Implementation Details

### execDelete Implementation
```go
func execDelete(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, cfg Config, fsys fs.FS, env map[string]string, flags *flag.FlagSet, args []string) error {
    if len(args) == 0 {
        return errWorktreeNameRequired
    }
    
    name := args[0]
    force, _ := flags.GetBool("force")
    withBranch, _ := flags.GetBool("with-branch")
    
    // 1. Verify git repository
    repoRoot, err := gitRepoRoot(cfg.EffectiveCwd)
    if err != nil {
        return fmt.Errorf("not a git repository")
    }
    
    // 2. Find worktree by name
    baseDir := resolveWorktreeBaseDir(cfg, repoRoot)
    wtPath := filepath.Join(baseDir, name)
    
    info, err := readWorktreeInfo(fsys, wtPath)
    if err != nil {
        // Use errors.Is() not os.IsNotExist() per TECH_SPEC
        if errors.Is(err, os.ErrNotExist) {
            return fmt.Errorf("worktree not found: %s", name)
        }
        return fmt.Errorf("reading worktree info: %w", err)
    }
    
    // 3. Check for uncommitted changes
    if !force {
        dirty, err := gitIsDirty(wtPath)
        if err != nil {
            return fmt.Errorf("checking worktree status: %w", err)
        }
        if dirty {
            return fmt.Errorf("worktree has uncommitted changes (use --force to override)")
        }
    }
    
    // 4. Run pre-delete hook
    hookRunner := NewHookRunner(fsys, repoRoot, env, stdout, stderr)
    if err := hookRunner.RunPreDelete(ctx, info, wtPath, cfg.EffectiveCwd); err != nil {
        return err  // Hook failure aborts deletion
    }
    
    // 5. Remove worktree
    if err := gitWorktreeRemove(repoRoot, wtPath, force); err != nil {
        return fmt.Errorf("removing worktree: %w", err)
    }
    
    // 6. Determine branch deletion
    deleteBranch := withBranch
    if !withBranch {
        // Prompt if interactive (stdin is a terminal)
        if IsTerminal() && stdin != nil {
            deleteBranch = promptYesNo(stdin, stdout, 
                fmt.Sprintf("Delete branch '%s'? (y/N) ", name))
        }
        // Non-interactive: keep branch (deleteBranch stays false)
    }
    
    // 7. Delete branch if requested
    if deleteBranch {
        if err := gitBranchDelete(repoRoot, name, force); err != nil {
            // Log but don't fail - worktree already deleted
            fprintf(stderr, "warning: could not delete branch %s: %v\n", name, err)
        }
    }
    
    // 8. Prune worktree metadata
    if err := gitWorktreePrune(repoRoot); err != nil {
        fprintf(stderr, "warning: git worktree prune failed: %v\n", err)
    }
    
    // 9. Output success
    if deleteBranch {
        fprintln(stdout, "Deleted worktree and branch:", name)
    } else {
        fprintln(stdout, "Deleted worktree:", name)
    }
    
    return nil
}
```

### Interactive Prompt Helper
```go
// promptYesNo prompts the user for yes/no confirmation.
// Returns true for 'y' or 'Y', false otherwise.
func promptYesNo(stdin io.Reader, stdout io.Writer, prompt string) bool {
    fprintf(stdout, "%s", prompt)
    
    reader := bufio.NewReader(stdin)
    response, _ := reader.ReadString('\n')
    
    return strings.ToLower(strings.TrimSpace(response)) == "y"
}
```

## Behavior Notes (per SPEC)

### Branch Deletion Logic
| Condition | Behavior |
|-----------|----------|
| --with-branch provided | Delete branch |
| Interactive terminal (tty) | Prompt user |
| Non-interactive | Keep branch |

### Error Cases
| Condition | Behavior |
|-----------|----------|
| Worktree not found | Exit with error |
| Uncommitted changes without --force | Exit with error |
| Hook fails (non-zero exit) | Abort, exit with error |

## Output Format (per SPEC)

### Success (worktree only)
```
Deleted worktree: swift-fox
```

### Success (with branch)
```
Deleted worktree and branch: swift-fox
```

### Interactive Prompt
```
Delete branch 'swift-fox'? (y/N)
```

## Acceptance Criteria
- Deletes worktree by name
- Errors if worktree not found
- Errors if uncommitted changes without --force
- --force bypasses dirty check
- Runs pre-delete hook before removal
- Aborts if hook fails
- --with-branch deletes branch
- Interactive prompt for branch deletion (tty only)
- Non-interactive keeps branch by default
- Runs git worktree prune after deletion

## Testing
Tests can use `RunWithInput()` to provide stdin for interactive prompts:

```go
// Test with "yes" response to prompt
stdout, stderr, code := c.RunWithInput([]string{"y"}, "delete", "my-worktree")

// Test with "no" response
stdout, stderr, code := c.RunWithInput([]string{"n"}, "delete", "my-worktree")

// Test non-interactive (nil stdin, IsTerminal() returns false in tests)
stdout, stderr, code := c.Run("delete", "my-worktree", "--with-branch")
```
