---
schema_version: 1
id: d5fay38
status: closed
closed: 2026-01-07T21:58:21Z
blocked-by: [d5fawvr, d5faxbg, d5fayh0]
created: 2026-01-07T19:07:57Z
type: task
priority: 2
---
# E2E tests for hooks with environment variables

## Overview
Add comprehensive tests verifying hook environment variables are set correctly.

## Background & Rationale
Hooks depend on WT_* environment variables being set correctly. These tests verify each variable is accessible and contains the expected value.

## Test Cases

```go
func TestHook_EnvironmentVariables(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create hook that outputs all WT_* variables to a file
    hookScript := `#!/bin/bash
cat > "$WT_PATH/hook-env.txt" << EOF
WT_ID=$WT_ID
WT_AGENT_ID=$WT_AGENT_ID
WT_NAME=$WT_NAME
WT_PATH=$WT_PATH
WT_BASE_BRANCH=$WT_BASE_BRANCH
WT_REPO_ROOT=$WT_REPO_ROOT
WT_SOURCE=$WT_SOURCE
EOF
`
    c.WriteFile(".wt/hooks/post-create", hookScript)
    c.Chmod(".wt/hooks/post-create", 0o755)
    
    createOut := c.MustRun("create", "--name", "env-test")
    wtPath := extractPath(createOut)
    
    // Read the env file created by hook
    envContent := c.ReadFileAt(wtPath, "hook-env.txt")
    
    // Verify each variable
    AssertContains(t, envContent, "WT_ID=1")
    AssertContains(t, envContent, "WT_NAME=env-test")
    AssertContains(t, envContent, "WT_BASE_BRANCH=main")
    AssertContains(t, envContent, "WT_REPO_ROOT="+c.Dir)
    AssertContains(t, envContent, "WT_SOURCE="+c.Dir)
    AssertContains(t, envContent, "WT_PATH="+wtPath)
    // WT_AGENT_ID will be some adjective-animal combo
    AssertContains(t, envContent, "WT_AGENT_ID=")
}

func TestHook_PreDeleteEnvVariables(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Create pre-delete hook that writes env to repo root
    hookScript := `#!/bin/bash
cat > "$WT_REPO_ROOT/pre-delete-env.txt" << EOF
WT_ID=$WT_ID
WT_NAME=$WT_NAME
EOF
`
    c.WriteFile(".wt/hooks/pre-delete", hookScript)
    c.Chmod(".wt/hooks/pre-delete", 0o755)
    
    c.MustRun("create", "--name", "delete-env-test")
    c.MustRun("delete", "delete-env-test", "--with-branch")
    
    // Check env was captured before deletion
    envContent := c.ReadFile("pre-delete-env.txt")
    AssertContains(t, envContent, "WT_ID=1")
    AssertContains(t, envContent, "WT_NAME=delete-env-test")
}

func TestHook_WorkingDirectory(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Hook that records pwd
    hookScript := `#!/bin/bash
pwd > "$WT_PATH/hook-pwd.txt"
`
    c.WriteFile(".wt/hooks/post-create", hookScript)
    c.Chmod(".wt/hooks/post-create", 0o755)
    
    createOut := c.MustRun("create", "--name", "pwd-test")
    wtPath := extractPath(createOut)
    
    pwd := strings.TrimSpace(c.ReadFileAt(wtPath, "hook-pwd.txt"))
    
    // Hook should run from WT_SOURCE (where wt was invoked)
    if pwd != c.Dir {
        t.Errorf("hook pwd = %s, want %s", pwd, c.Dir)
    }
}

func TestHook_Timeout(t *testing.T) {
    // This test is slow, mark appropriately
    if testing.Short() {
        t.Skip("skipping slow test")
    }
    
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Hook that sleeps longer than timeout
    // Note: actual timeout is 5 min, so we can't really test this in CI
    // Instead, test that a quick hook works and a slow one could be killed
    hookScript := `#!/bin/bash
sleep 1
touch "$WT_PATH/hook-ran"
`
    c.WriteFile(".wt/hooks/post-create", hookScript)
    c.Chmod(".wt/hooks/post-create", 0o755)
    
    createOut := c.MustRun("create", "--name", "timeout-test")
    wtPath := extractPath(createOut)
    
    // Verify hook completed
    if !c.FileExistsAt(wtPath, "hook-ran") {
        t.Error("hook should have completed")
    }
}
```

### Helper Methods
```go
func (c *CLI) ReadFileAt(baseDir, relPath string) string {
    c.t.Helper()
    path := filepath.Join(baseDir, relPath)
    content, err := os.ReadFile(path)
    if err != nil {
        c.t.Fatalf("failed to read %s: %v", path, err)
    }
    return string(content)
}

func (c *CLI) FileExistsAt(baseDir, relPath string) bool {
    path := filepath.Join(baseDir, relPath)
    _, err := os.Stat(path)
    return err == nil
}
```

## Acceptance Criteria
- All WT_* variables are tested
- post-create hook variables verified
- pre-delete hook variables verified
- Hook working directory is WT_SOURCE
- Hook can access all expected paths
