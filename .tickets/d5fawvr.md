---
schema_version: 1
id: d5fawvr
status: open
blocked-by: [d5faw58]
created: 2026-01-07T19:05:19Z
type: task
priority: 2
---
# E2E tests for wt create command

## Overview
Implement comprehensive E2E tests for the create command using real git operations.

## Background & Rationale
Per TECH_SPEC.md: "Always write e2e tests, never write unit tests. Tests should run with the real git binary in a tmp dir."

The existing test infrastructure (CLI helper in testing_test.go) provides a good foundation but needs enhancement to work with real git repos.

## Current State
- testing_test.go has CLI helper with InitGitRepo() that creates minimal .git structure
- This is insufficient for real git operations (worktree add, etc.)
- Need to use actual `git init` and `git commit`

## Implementation Details

### Enhanced Test Helper
```go
// InitRealGitRepo initializes a real git repository with an initial commit.
func (c *CLI) InitRealGitRepo() {
    c.t.Helper()
    
    // git init
    cmd := exec.Command("git", "init")
    cmd.Dir = c.Dir
    if out, err := cmd.CombinedOutput(); err != nil {
        c.t.Fatalf("git init failed: %v\n%s", err, out)
    }
    
    // Configure git user (required for commits)
    cmd = exec.Command("git", "config", "user.email", "test@test.com")
    cmd.Dir = c.Dir
    cmd.Run()
    cmd = exec.Command("git", "config", "user.name", "Test User")
    cmd.Dir = c.Dir
    cmd.Run()
    
    // Create initial file and commit
    c.WriteFile("README.md", "# Test Repo")
    
    cmd = exec.Command("git", "add", ".")
    cmd.Dir = c.Dir
    cmd.Run()
    
    cmd = exec.Command("git", "commit", "-m", "Initial commit")
    cmd.Dir = c.Dir
    if out, err := cmd.CombinedOutput(); err != nil {
        c.t.Fatalf("git commit failed: %v\n%s", err, out)
    }
}
```

### Test Cases

```go
func TestCreate_Basic(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    stdout := c.MustRun("create")
    
    // Verify output format
    AssertContains(t, stdout, "Created worktree:")
    AssertContains(t, stdout, "name:")
    AssertContains(t, stdout, "agent_id:")
    AssertContains(t, stdout, "id:")
    AssertContains(t, stdout, "path:")
    AssertContains(t, stdout, "branch:")
    AssertContains(t, stdout, "from:")
}

func TestCreate_WithCustomName(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    stdout := c.MustRun("create", "--name", "my-feature")
    
    AssertContains(t, stdout, "name:        my-feature")
    AssertContains(t, stdout, "branch:      my-feature")
}

func TestCreate_FromBranch(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create a develop branch
    cmd := exec.Command("git", "branch", "develop")
    cmd.Dir = c.Dir
    cmd.Run()
    
    stdout := c.MustRun("create", "--from-branch", "develop")
    
    AssertContains(t, stdout, "from:        develop")
}

func TestCreate_IncrementingID(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create first worktree
    stdout1 := c.MustRun("create")
    AssertContains(t, stdout1, "id:          1")
    
    // Create second worktree
    stdout2 := c.MustRun("create")
    AssertContains(t, stdout2, "id:          2")
}

func TestCreate_NotInGitRepo(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    // Don't init git repo
    
    stderr := c.MustFail("create")
    
    AssertContains(t, stderr, "not a git repository")
}

func TestCreate_NameCollision(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create first worktree with custom name
    c.MustRun("create", "--name", "my-feature")
    
    // Try to create second with same name
    stderr := c.MustFail("create", "--name", "my-feature")
    
    AssertContains(t, stderr, "already in use")
}

func TestCreate_HookSuccess(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create hook that creates a marker file
    c.WriteFile(".wt/hooks/post-create", "#!/bin/bash\ntouch \"\/hook-ran\"\n")
    c.Chmod(".wt/hooks/post-create", 0o755)
    
    c.MustRun("create", "--name", "test-wt")
    
    // Verify hook ran (marker file exists in worktree)
    // Need to check in worktree directory
}

func TestCreate_HookFailure(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create hook that fails
    c.WriteFile(".wt/hooks/post-create", "#!/bin/bash\nexit 1\n")
    c.Chmod(".wt/hooks/post-create", 0o755)
    
    stderr := c.MustFail("create", "--name", "test-wt")
    
    AssertContains(t, stderr, "hook")
    // Verify rollback: worktree should not exist
}
```

### Helper for Chmod
```go
func (c *CLI) Chmod(relPath string, mode os.FileMode) {
    c.t.Helper()
    path := filepath.Join(c.Dir, relPath)
    if err := os.Chmod(path, mode); err != nil {
        c.t.Fatalf("chmod %s failed: %v", relPath, err)
    }
}
```

## Acceptance Criteria
- All tests use real git commands
- Tests run in isolated temp directories
- Tests are parallelizable (t.Parallel())
- Tests cover:
  - Basic creation with defaults
  - Custom name (--name)
  - From specific branch (--from-branch)
  - Incrementing IDs
  - Error: not in git repo
  - Error: name collision
  - Hook success
  - Hook failure with rollback

## Test Helper (implement in testing_test.go)

Add these helpers when implementing the first E2E test:

```go
// extractPath extracts the path from wt create output.
func extractPath(createOutput string) string {
    return extractField(createOutput, "path")
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
