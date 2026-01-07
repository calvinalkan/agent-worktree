---
schema_version: 1
id: d5fax0r
status: closed
closed: 2026-01-07T21:20:24Z
blocked-by: [d5fawb0, d5faw58]
created: 2026-01-07T19:05:39Z
type: task
priority: 2
---
# E2E tests for wt list command

## Overview
Implement comprehensive E2E tests for the list command.

## Background & Rationale
Test the list command with real git worktrees to ensure correct discovery and output formatting.

## Test Cases

```go
func TestList_Empty(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    stdout, _, code := c.Run("list")
    
    if code != 0 {
        t.Errorf("expected exit code 0, got %d", code)
    }
    // Empty list = no output (or just header?)
    if strings.TrimSpace(stdout) != "" {
        t.Errorf("expected empty output, got: %s", stdout)
    }
}

func TestList_SingleWorktree(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create a worktree first
    c.MustRun("create", "--name", "test-wt")
    
    stdout := c.MustRun("list")
    
    AssertContains(t, stdout, "NAME")
    AssertContains(t, stdout, "PATH")
    AssertContains(t, stdout, "CREATED")
    AssertContains(t, stdout, "test-wt")
}

func TestList_MultipleWorktrees(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "wt-one")
    c.MustRun("create", "--name", "wt-two")
    c.MustRun("create", "--name", "wt-three")
    
    stdout := c.MustRun("list")
    
    AssertContains(t, stdout, "wt-one")
    AssertContains(t, stdout, "wt-two")
    AssertContains(t, stdout, "wt-three")
}

func TestList_JSON(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "json-test")
    
    stdout := c.MustRun("list", "--json")
    
    // Parse as JSON to verify structure
    var worktrees []struct {
        Name       string  `json:"name"`
        AgentID    *string `json:"agent_id"`
        ID         *int    `json:"id"`
        Path       string  `json:"path"`
        BaseBranch string  `json:"base_branch"`
        Created    *string `json:"created"`
    }
    
    if err := json.Unmarshal([]byte(stdout), &worktrees); err != nil {
        t.Fatalf("invalid JSON output: %v\n%s", err, stdout)
    }
    
    if len(worktrees) != 1 {
        t.Errorf("expected 1 worktree, got %d", len(worktrees))
    }
    
    if worktrees[0].Name != "json-test" {
        t.Errorf("expected name json-test, got %s", worktrees[0].Name)
    }
}

func TestList_NotInGitRepo(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    // Don't init git repo
    
    stderr := c.MustFail("list")
    
    AssertContains(t, stderr, "not a git repository")
}
```

## Acceptance Criteria
- Test empty worktree list
- Test single worktree
- Test multiple worktrees
- Test JSON output format and structure
- Test error when not in git repo
