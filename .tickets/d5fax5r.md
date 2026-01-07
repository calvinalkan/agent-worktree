---
schema_version: 1
id: d5fax5r
status: closed
closed: 2026-01-07T21:52:39Z
blocked-by: [d5fawfg, d5faw58, d5fayh0]
created: 2026-01-07T19:05:59Z
type: task
priority: 2
---
# E2E tests for wt info command

## Overview
Implement comprehensive E2E tests for the info command.

## Background & Rationale
The info command requires being run from within a wt-managed worktree. Tests need to create worktrees and then run commands from within them.

## Implementation Notes
The CLI helper's --cwd flag allows running commands as if from a different directory. This enables testing info without actually cd'ing.

## Test Cases

```go
func TestInfo_Basic(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create worktree
    createOut := c.MustRun("create", "--name", "info-test")
    
    // Extract worktree path from create output
    wtPath := extractPath(createOut)
    
    // Run info from worktree directory
    c2 := NewCLITesterAt(t, wtPath)
    stdout := c2.MustRun("info")
    
    AssertContains(t, stdout, "name:        info-test")
    AssertContains(t, stdout, "agent_id:")
    AssertContains(t, stdout, "id:")
    AssertContains(t, stdout, "path:")
    AssertContains(t, stdout, "base_branch:")
    AssertContains(t, stdout, "created:")
}

func TestInfo_JSON(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    createOut := c.MustRun("create", "--name", "json-info-test")
    wtPath := extractPath(createOut)
    
    c2 := NewCLITesterAt(t, wtPath)
    stdout := c2.MustRun("info", "--json")
    
    var info struct {
        Name       string `json:"name"`
        AgentID    string `json:"agent_id"`
        ID         int    `json:"id"`
        Path       string `json:"path"`
        BaseBranch string `json:"base_branch"`
        Created    string `json:"created"`
    }
    
    if err := json.Unmarshal([]byte(stdout), &info); err != nil {
        t.Fatalf("invalid JSON: %v\n%s", err, stdout)
    }
    
    if info.Name != "json-info-test" {
        t.Errorf("expected name json-info-test, got %s", info.Name)
    }
}

func TestInfo_Field(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "field-test")
    // ... get worktree path and run from there
    
    tests := []struct {
        field string
        want  string
    }{
        {"name", "field-test"},
        {"id", "1"},
        {"base_branch", "main"},
    }
    
    for _, tt := range tests {
        stdout := c2.MustRun("info", "--field", tt.field)
        if strings.TrimSpace(stdout) != tt.want {
            t.Errorf("--field %s: got %q, want %q", tt.field, stdout, tt.want)
        }
    }
}

func TestInfo_FieldInvalid(t *testing.T) {
    t.Parallel()
    
    // ... setup worktree ...
    
    stderr := c2.MustFail("info", "--field", "invalid")
    AssertContains(t, stderr, "unknown field")
}

func TestInfo_NotInWorktree(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    // Don't create any worktree
    
    stderr := c.MustFail("info")
    AssertContains(t, stderr, "not in a wt-managed worktree")
}

func TestInfo_FromSubdirectory(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    createOut := c.MustRun("create", "--name", "subdir-test")
    wtPath := extractPath(createOut)
    
    // Create subdirectory in worktree
    subDir := filepath.Join(wtPath, "src", "pkg")
    os.MkdirAll(subDir, 0o755)
    
    // Run info from subdirectory
    c2 := NewCLITesterAt(t, subDir)
    stdout := c2.MustRun("info")
    
    AssertContains(t, stdout, "name:        subdir-test")
}
```

### Helper for Testing from Specific Directory
```go
// NewCLITesterAt creates a CLI tester that runs from a specific directory.
func NewCLITesterAt(t *testing.T, dir string) *CLI {
    t.Helper()
    return &CLI{
        t:   t,
        Dir: dir,
        Env: map[string]string{},
    }
}
```

## Acceptance Criteria
- Test basic info output
- Test JSON output format
- Test --field for each valid field
- Test --field with invalid field
- Test error when not in worktree
- Test info from subdirectory of worktree
