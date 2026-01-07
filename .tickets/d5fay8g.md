---
schema_version: 1
id: d5fay8g
status: closed
closed: 2026-01-07T21:45:15Z
blocked-by: [d5faxtg, d5faw58, d5fawb0]
created: 2026-01-07T19:08:18Z
type: task
priority: 2
---
# E2E tests for configuration system

## Overview
Add E2E tests for the configuration loading and precedence system.

## Background & Rationale
The config system has specific precedence rules:
1. --config flag (explicit path)
2. Project config: .wt/config.json
3. User config: ~/.config/wt/config.json
4. Built-in defaults

Tests need to verify these rules work correctly.

## Test Cases

```go
func TestConfig_Defaults(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create worktree with defaults
    stdout := c.MustRun("create", "--name", "default-cfg")
    
    // Default base is ~/code/worktrees
    // Worktree should be at ~/code/worktrees/<repo-name>/default-cfg
    // For test, we verify the path contains the expected pattern
    AssertContains(t, stdout, "worktrees")
}

func TestConfig_ProjectConfig(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create project config
    c.WriteFile(".wt/config.json", `{"base": "./local-wt"}`)
    
    stdout := c.MustRun("create", "--name", "project-cfg")
    
    // With relative base, worktree should be at ./local-wt/project-cfg
    AssertContains(t, stdout, "local-wt")
}

func TestConfig_ExplicitFlag(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create explicit config file
    c.WriteFile("custom.json", `{"base": "./custom-wt"}`)
    
    // Also create project config that should be ignored
    c.WriteFile(".wt/config.json", `{"base": "./project-wt"}`)
    
    stdout := c.MustRun("--config", "custom.json", "create", "--name", "explicit-cfg")
    
    // Explicit config should win
    AssertContains(t, stdout, "custom-wt")
    AssertNotContains(t, stdout, "project-wt")
}

func TestConfig_InvalidJSON(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create invalid JSON config
    c.WriteFile("bad.json", `{invalid json}`)
    
    stderr := c.MustFail("--config", "bad.json", "list")
    AssertContains(t, stderr, "parsing config")
}

func TestConfig_RelativeBasePath(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Relative base path: worktrees created relative to cwd, no repo name
    c.WriteFile(".wt/config.json", `{"base": "../sibling-wt"}`)
    
    stdout := c.MustRun("create", "--name", "relative-test")
    
    // Path should NOT include repo name for relative base
    AssertContains(t, stdout, "sibling-wt/relative-test")
}

func TestConfig_AbsoluteBasePath(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Absolute base path: worktrees include repo name
    tmpDir := t.TempDir()
    c.WriteFile(".wt/config.json", fmt.Sprintf(`{"base": "%s"}`, tmpDir))
    
    stdout := c.MustRun("create", "--name", "absolute-test")
    
    // Path should include repo name for absolute base
    repoName := filepath.Base(c.Dir)
    AssertContains(t, stdout, filepath.Join(tmpDir, repoName, "absolute-test"))
}

func TestConfig_TildeExpansion(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Can't easily test ~ expansion since we'd write to real home
    // But we can verify it doesn't crash
    c.WriteFile(".wt/config.json", `{"base": "~/test-worktrees"}`)
    
    // At minimum, this should parse without error
    _, _, code := c.Run("list")
    if code != 0 {
        // List with no worktrees should still succeed
        t.Error("expected success with ~ in path")
    }
}
```

## Acceptance Criteria
- Test default config behavior
- Test project config (.wt/config.json)
- Test explicit --config flag precedence
- Test invalid JSON error handling
- Test relative vs absolute base path behavior
- Test ~ expansion
