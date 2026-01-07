---
schema_version: 1
id: d5faxbg
status: open
blocked-by: [d5fawn0, d5faw58, d5fawb0, d5fayh0]
created: 2026-01-07T19:06:22Z
type: task
priority: 2
---
# E2E tests for wt delete command

## Overview
Implement comprehensive E2E tests for the delete command.

## Background & Rationale
Delete has the most complex behavior: dirty checks, hooks, branch deletion, and interactive prompts. Need thorough coverage.

## Test Cases

```go
func TestDelete_Basic(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "to-delete")
    
    // Verify worktree exists in list
    stdout := c.MustRun("list")
    AssertContains(t, stdout, "to-delete")
    
    // Delete it
    stdout = c.MustRun("delete", "to-delete", "--with-branch")
    AssertContains(t, stdout, "Deleted worktree and branch: to-delete")
    
    // Verify it's gone
    stdout = c.MustRun("list")
    AssertNotContains(t, stdout, "to-delete")
}

func TestDelete_NotFound(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    stderr := c.MustFail("delete", "nonexistent")
    AssertContains(t, stderr, "worktree not found")
}

func TestDelete_DirtyWorktree(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    createOut := c.MustRun("create", "--name", "dirty-wt")
    wtPath := extractPath(createOut)
    
    // Create uncommitted change in worktree
    os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("uncommitted"), 0o644)
    
    // Try to delete without --force
    stderr := c.MustFail("delete", "dirty-wt")
    AssertContains(t, stderr, "uncommitted changes")
    AssertContains(t, stderr, "--force")
}

func TestDelete_DirtyWorktreeForce(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    createOut := c.MustRun("create", "--name", "dirty-force")
    wtPath := extractPath(createOut)
    
    // Create uncommitted change
    os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("uncommitted"), 0o644)
    
    // Delete with --force
    stdout := c.MustRun("delete", "dirty-force", "--force", "--with-branch")
    AssertContains(t, stdout, "Deleted worktree and branch: dirty-force")
}

func TestDelete_WithoutBranch(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "keep-branch")
    
    // Delete without --with-branch (non-interactive keeps branch)
    stdout := c.MustRun("delete", "keep-branch")
    AssertContains(t, stdout, "Deleted worktree: keep-branch")
    AssertNotContains(t, stdout, "and branch")
    
    // Verify branch still exists
    cmd := exec.Command("git", "-C", c.Dir, "branch", "--list", "keep-branch")
    out, _ := cmd.Output()
    if !strings.Contains(string(out), "keep-branch") {
        t.Error("branch should still exist")
    }
}

func TestDelete_PreDeleteHookSuccess(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create hook that logs
    c.WriteFile(".wt/hooks/pre-delete", "#!/bin/bash\necho \"pre-delete ran\" >&2\n")
    c.Chmod(".wt/hooks/pre-delete", 0o755)
    
    c.MustRun("create", "--name", "hook-test")
    
    stdout, stderr, code := c.Run("delete", "hook-test", "--with-branch")
    if code != 0 {
        t.Errorf("expected success, got code %d", code)
    }
    AssertContains(t, stderr, "pre-delete ran")
    AssertContains(t, stdout, "Deleted")
}

func TestDelete_PreDeleteHookFailure(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create hook that fails
    c.WriteFile(".wt/hooks/pre-delete", "#!/bin/bash\nexit 1\n")
    c.Chmod(".wt/hooks/pre-delete", 0o755)
    
    c.MustRun("create", "--name", "hook-fail")
    
    stderr := c.MustFail("delete", "hook-fail")
    AssertContains(t, stderr, "hook")
    
    // Verify worktree still exists (deletion aborted)
    stdout := c.MustRun("list")
    AssertContains(t, stdout, "hook-fail")
}

func TestDelete_NoArgument(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    stderr := c.MustFail("delete")
    AssertContains(t, stderr, "worktree name is required")
}

func TestDelete_InteractivePromptYes(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "prompt-yes")
    
    // Use RunWithInput to simulate "y" response
    // Note: IsTerminal() returns false in tests, so this tests the prompt logic
    // when stdin is explicitly provided. The actual tty detection is separate.
    stdout, _, code := c.RunWithInput([]string{"y"}, "delete", "prompt-yes")
    
    if code != 0 {
        t.Errorf("expected success, got code %d", code)
    }
    // In non-tty mode, branch is kept by default regardless of stdin
    AssertContains(t, stdout, "Deleted worktree:")
}

func TestDelete_InteractivePromptNo(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "prompt-no")
    
    // Use RunWithInput to simulate "n" response
    stdout, _, code := c.RunWithInput([]string{"n"}, "delete", "prompt-no")
    
    if code != 0 {
        t.Errorf("expected success, got code %d", code)
    }
    AssertContains(t, stdout, "Deleted worktree:")
    AssertNotContains(t, stdout, "and branch")
}
```

## Helper: Chmod
Add this helper to testing_test.go if not present:

```go
func (c *CLI) Chmod(relPath string, mode os.FileMode) {
    c.t.Helper()
    path := filepath.Join(c.Dir, relPath)
    if err := os.Chmod(path, mode); err != nil {
        c.t.Fatalf("chmod %s failed: %v", relPath, err)
    }
}
```

## Note on Interactive Testing
`IsTerminal()` checks `os.Stdin.Stat()` which returns false in test environments 
(no tty). This means:
- Non-interactive behavior (keep branch) is the default in tests
- To test the prompt logic itself, you'd need to mock `IsTerminal()` or test 
  `promptYesNo()` directly
- The `--with-branch` flag bypasses the prompt entirely

## Acceptance Criteria
- Test basic deletion
- Test worktree not found error
- Test dirty worktree without --force (error)
- Test dirty worktree with --force (success)
- Test deletion without --with-branch (keeps branch)
- Test deletion with --with-branch (deletes branch)
- Test pre-delete hook success
- Test pre-delete hook failure (aborts deletion)
- Test missing argument error
