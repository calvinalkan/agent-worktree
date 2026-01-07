---
schema_version: 1
id: d5fb6v8
status: open
blocked-by: [d5faw58, d5fawb0, d5fawfg, d5fawn0]
created: 2026-01-07T20:15:00Z
type: task
priority: 2
---
# Add tests for argument validation and error messages

## Overview
Add comprehensive tests for argument validation, unknown flags, and helpful error messages across all commands.

## Background & Rationale
While the command framework (command.go) handles basic flag parsing errors, and individual command tickets cover command-specific errors, there's no unified test suite verifying:
- Unknown flags produce helpful errors
- Global flag validation works correctly
- Error messages are consistent and actionable

## Test Cases

### Unknown Flag Handling
```go
func TestUnknownFlags(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name string
        args []string
        want string
    }{
        {"create unknown flag", []string{"create", "--bogus"}, "unknown flag: --bogus"},
        {"list unknown flag", []string{"list", "--foo"}, "unknown flag: --foo"},
        {"info unknown flag", []string{"info", "--bar"}, "unknown flag: --bar"},
        {"delete unknown flag", []string{"delete", "x", "--baz"}, "unknown flag: --baz"},
        {"global unknown flag", []string{"--unknown", "list"}, "unknown flag: --unknown"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            c := NewCLITester(t)
            c.InitRealGitRepo()
            
            stderr := c.MustFail(tt.args...)
            AssertContains(t, stderr, tt.want)
            // Should also show help
            AssertContains(t, stderr, "Usage:")
        })
    }
}
```

### Global Flag Validation
```go
func TestGlobalFlagValidation(t *testing.T) {
    t.Parallel()
    
    t.Run("--cwd nonexistent directory", func(t *testing.T) {
        t.Parallel()
        c := NewCLITester(t)
        
        _, stderr, code := c.Run("--cwd", "/nonexistent/path", "list")
        
        // Should fail gracefully
        if code == 0 {
            t.Error("expected failure for nonexistent --cwd")
        }
        AssertContains(t, stderr, "error")
    })
    
    t.Run("--config nonexistent file", func(t *testing.T) {
        t.Parallel()
        c := NewCLITester(t)
        c.InitRealGitRepo()
        
        _, stderr, code := c.Run("--config", "nonexistent.json", "list")
        
        if code == 0 {
            t.Error("expected failure for nonexistent config")
        }
        AssertContains(t, stderr, "error")
    })
    
    t.Run("--config invalid JSON", func(t *testing.T) {
        t.Parallel()
        c := NewCLITester(t)
        c.WriteFile("bad.json", "{invalid}")
        c.InitRealGitRepo()
        
        stderr := c.MustFail("--config", "bad.json", "list")
        AssertContains(t, stderr, "parsing config")
    })
}
```

### Error Message Format Consistency
```go
func TestErrorMessageFormat(t *testing.T) {
    t.Parallel()
    
    // All errors should start with "error:" for consistent parsing
    tests := []struct {
        name string
        args []string
    }{
        {"unknown command", []string{"boguscmd"}},
        {"delete no arg", []string{"delete"}},
        {"info outside worktree", []string{"info"}},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            c := NewCLITester(t)
            c.InitRealGitRepo()
            
            _, stderr, code := c.Run(tt.args...)
            
            if code == 0 {
                t.Fatal("expected failure")
            }
            // All errors should have consistent format
            AssertContains(t, stderr, "error:")
        })
    }
}
```

## Acceptance Criteria
- Unknown flags for all commands produce clear errors with usage
- Global flag errors are handled gracefully
- Error messages consistently start with "error:"
- Errors suggest how to get help where appropriate
- Tests cover both global and command-specific flag errors

## Testing Location
Add to run_test.go alongside existing help tests
