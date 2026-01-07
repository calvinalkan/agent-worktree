---
schema_version: 1
id: d5favpr
status: closed
closed: 2026-01-07T20:36:14Z
blocked-by: [d5fav78]
created: 2026-01-07T19:02:51Z
type: task
priority: 1
---
# Implement hook execution system

## Overview
Implement the hook execution system for post-create and pre-delete hooks.

## Background & Rationale
Per SPEC.md, hooks enable project-specific automation:
- **post-create**: Run after worktree creation (npm install, docker compose up, etc.)
- **pre-delete**: Run before worktree deletion (docker compose down, cleanup, etc.)

Hooks are stored in the main repository's .wt/hooks/ directory and use shebang for interpreter selection.

## Current State
No hook execution logic exists.

## Implementation Details

### Hook Environment Variables
```go
// hookEnv creates the WT_* environment variables available to hooks.
func hookEnv(info WorktreeInfo, wtPath, repoRoot, sourceDir string) map[string]string {
    return map[string]string{
        "WT_ID":          strconv.Itoa(info.ID),
        "WT_AGENT_ID":    info.AgentID,
        "WT_NAME":        info.Name,
        "WT_PATH":        wtPath,
        "WT_BASE_BRANCH": info.BaseBranch,
        "WT_REPO_ROOT":   repoRoot,
        "WT_SOURCE":      sourceDir,
    }
}
```

### Hook Execution
```go
// runHook executes a hook script if it exists.
// hookName is "post-create" or "pre-delete".
// baseEnv is the inherited environment (passed from Run()'s env parameter).
// wtEnv contains the WT_* variables to add.
// Returns nil if hook doesn't exist.
// Returns error if hook exists but is not executable, or if execution fails.
func runHook(ctx context.Context, fsys fs.FS, repoRoot string, hookName string, baseEnv, wtEnv map[string]string, cwd string, stdout, stderr io.Writer) error {
    hookPath := filepath.Join(repoRoot, ".wt", "hooks", hookName)
    
    // Check if hook exists
    info, err := fsys.Stat(hookPath)
    if err != nil {
        // Use errors.Is() not os.IsNotExist() per TECH_SPEC
        if errors.Is(err, os.ErrNotExist) {
            return nil  // Hook doesn't exist, skip silently
        }
        return fmt.Errorf("checking hook %s: %w", hookName, err)
    }
    
    // Check if executable
    if info.Mode()&0o111 == 0 {
        return fmt.Errorf("hook %s exists but is not executable", hookPath)
    }
    
    // Build command with timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    cmd := exec.CommandContext(timeoutCtx, hookPath)
    cmd.Dir = cwd
    cmd.Stdout = stdout
    cmd.Stderr = stderr
    
    // Build environment from baseEnv + wtEnv
    // Note: baseEnv comes from Run()'s env parameter, not os.Environ()
    // This respects TECH_SPEC's env abstraction requirement
    cmd.Env = make([]string, 0, len(baseEnv)+len(wtEnv))
    for k, v := range baseEnv {
        cmd.Env = append(cmd.Env, k+"="+v)
    }
    for k, v := range wtEnv {
        cmd.Env = append(cmd.Env, k+"="+v)
    }
    
    if err := cmd.Run(); err != nil {
        if timeoutCtx.Err() == context.DeadlineExceeded {
            return fmt.Errorf("hook %s timed out after 5 minutes", hookName)
        }
        return fmt.Errorf("hook %s failed: %w", hookName, err)
    }
    
    return nil
}
```

### Hook Runner Type
```go
// HookRunner executes lifecycle hooks for worktrees.
type HookRunner struct {
    fsys     fs.FS
    repoRoot string
    baseEnv  map[string]string  // inherited environment from Run()
    stdout   io.Writer
    stderr   io.Writer
}

// NewHookRunner creates a hook runner.
// baseEnv should be the env map passed to Run() - we don't call os.Environ().
func NewHookRunner(fsys fs.FS, repoRoot string, baseEnv map[string]string, stdout, stderr io.Writer) *HookRunner {
    return &HookRunner{
        fsys:     fsys,
        repoRoot: repoRoot,
        baseEnv:  baseEnv,
        stdout:   stdout,
        stderr:   stderr,
    }
}

func (h *HookRunner) RunPostCreate(ctx context.Context, info WorktreeInfo, wtPath, sourceDir string) error {
    wtEnv := hookEnv(info, wtPath, h.repoRoot, sourceDir)
    return runHook(ctx, h.fsys, h.repoRoot, "post-create", h.baseEnv, wtEnv, sourceDir, h.stdout, h.stderr)
}

func (h *HookRunner) RunPreDelete(ctx context.Context, info WorktreeInfo, wtPath, sourceDir string) error {
    wtEnv := hookEnv(info, wtPath, h.repoRoot, sourceDir)
    return runHook(ctx, h.fsys, h.repoRoot, "pre-delete", h.baseEnv, wtEnv, sourceDir, h.stdout, h.stderr)
}
```

## TECH_SPEC Compliance Notes
- Uses `errors.Is(err, os.ErrNotExist)` instead of `os.IsNotExist(err)` 
- Does NOT call `os.Environ()` - instead receives baseEnv from caller
- The env map flows: main() → Run() → command → HookRunner
- This maintains the env abstraction for testability

## Hook Behavior per SPEC
| Condition | Behavior |
|-----------|----------|
| Hook doesn't exist | Skip silently (not an error) |
| Hook exists but not executable | Exit with error |
| Hook exits non-zero | Return error (caller handles rollback/abort) |
| Hook times out (5 min) | Kill and return error |

## Environment Variables (per SPEC)
| Variable | Description |
|----------|-------------|
| WT_ID | Unique worktree number |
| WT_AGENT_ID | Generated word combo identifier |
| WT_NAME | Worktree directory/branch name |
| WT_PATH | Absolute path to worktree |
| WT_BASE_BRANCH | Branch worktree was created from |
| WT_REPO_ROOT | Absolute path to main repository |
| WT_SOURCE | Absolute path to directory where command was invoked |

## Acceptance Criteria
- Hooks skip silently if not present
- Error if hook exists but not executable
- 5 minute timeout enforced
- All WT_* environment variables set correctly
- Hook cwd is effective cwd (where wt was invoked)
- Hook stdout/stderr passed through
- Does NOT use os.Environ() - uses passed env map

## Testing
- Test hook not present (should skip)
- Test hook not executable (should error)
- Test hook success
- Test hook failure (non-zero exit)
- Test environment variables are set
