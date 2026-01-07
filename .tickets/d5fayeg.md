---
schema_version: 1
id: d5fayeg
status: open
blocked-by: [d5fawvr, d5fax0r, d5fax5r, d5faxbg, d5fayh0]
created: 2026-01-07T19:08:42Z
type: task
priority: 2
---
# Integration test: full workflow end-to-end

## Overview
Add a comprehensive integration test that exercises the full workflow: create → list → info → delete.

## Background & Rationale
Individual command tests verify specific behaviors, but we also need to verify the commands work together correctly in a realistic workflow.

## Test Cases

```go
func TestFullWorkflow(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Step 1: Create first worktree
    createOut := c.MustRun("create", "--name", "feature-a")
    AssertContains(t, createOut, "name:        feature-a")
    AssertContains(t, createOut, "id:          1")
    wtPath := extractPath(createOut)
    
    // Step 2: Create second worktree from different branch
    cmd := exec.Command("git", "branch", "develop")
    cmd.Dir = c.Dir
    cmd.Run()
    
    createOut2 := c.MustRun("create", "--name", "feature-b", "--from-branch", "develop")
    AssertContains(t, createOut2, "name:        feature-b")
    AssertContains(t, createOut2, "id:          2")
    AssertContains(t, createOut2, "from:        develop")
    
    // Step 3: List should show both
    listOut := c.MustRun("list")
    AssertContains(t, listOut, "feature-a")
    AssertContains(t, listOut, "feature-b")
    
    // Step 4: Info from first worktree
    c2 := NewCLITesterAt(t, wtPath)
    infoOut := c2.MustRun("info")
    AssertContains(t, infoOut, "name:        feature-a")
    AssertContains(t, infoOut, "id:          1")
    
    // Step 5: Info JSON format
    infoJSON := c2.MustRun("info", "--json")
    var info struct {
        Name string `json:"name"`
        ID   int    `json:"id"`
    }
    json.Unmarshal([]byte(infoJSON), &info)
    if info.Name != "feature-a" || info.ID != 1 {
        t.Errorf("unexpected JSON: %+v", info)
    }
    
    // Step 6: Delete first worktree
    deleteOut := c.MustRun("delete", "feature-a", "--with-branch")
    AssertContains(t, deleteOut, "Deleted worktree and branch: feature-a")
    
    // Step 7: List should show only second
    listOut = c.MustRun("list")
    AssertNotContains(t, listOut, "feature-a")
    AssertContains(t, listOut, "feature-b")
    
    // Step 8: Create new worktree (should get ID 3, not reuse 1)
    createOut3 := c.MustRun("create", "--name", "feature-c")
    AssertContains(t, createOut3, "id:          3")
    
    // Step 9: Delete remaining worktrees
    c.MustRun("delete", "feature-b", "--with-branch")
    c.MustRun("delete", "feature-c", "--with-branch")
    
    // Step 10: List should be empty
    listOut = c.MustRun("list")
    if strings.TrimSpace(listOut) != "" {
        t.Errorf("expected empty list, got: %s", listOut)
    }
}

func TestWorkflow_WithHooks(t *testing.T) {
    t.Parallel()
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    // Setup hooks
    c.WriteFile(".wt/hooks/post-create", `#!/bin/bash
echo "CREATED: $WT_NAME" >> "$WT_REPO_ROOT/hook-log.txt"
`)
    c.WriteFile(".wt/hooks/pre-delete", `#!/bin/bash
echo "DELETING: $WT_NAME" >> "$WT_REPO_ROOT/hook-log.txt"
`)
    c.Chmod(".wt/hooks/post-create", 0o755)
    c.Chmod(".wt/hooks/pre-delete", 0o755)
    
    // Create and delete
    c.MustRun("create", "--name", "hook-wt")
    c.MustRun("delete", "hook-wt", "--with-branch")
    
    // Verify hooks ran in order
    log := c.ReadFile("hook-log.txt")
    lines := strings.Split(strings.TrimSpace(log), "\n")
    
    if len(lines) != 2 {
        t.Fatalf("expected 2 log lines, got %d: %v", len(lines), lines)
    }
    if !strings.Contains(lines[0], "CREATED: hook-wt") {
        t.Errorf("line 1 should be create: %s", lines[0])
    }
    if !strings.Contains(lines[1], "DELETING: hook-wt") {
        t.Errorf("line 2 should be delete: %s", lines[1])
    }
}

func TestWorkflow_ListJSON_ParseableByJq(t *testing.T) {
    t.Parallel()
    
    // Skip if jq not available
    if _, err := exec.LookPath("jq"); err != nil {
        t.Skip("jq not available")
    }
    
    c := NewCLITester(t)
    c.InitRealGitRepo()
    
    c.MustRun("create", "--name", "jq-test")
    
    listJSON := c.MustRun("list", "--json")
    
    // Pipe to jq
    cmd := exec.Command("jq", ".[0].name")
    cmd.Stdin = strings.NewReader(listJSON)
    out, err := cmd.Output()
    if err != nil {
        t.Fatalf("jq failed: %v", err)
    }
    
    if strings.TrimSpace(string(out)) != `"jq-test"` {
        t.Errorf("jq output = %s, want \"jq-test\"", out)
    }
}
```

## Acceptance Criteria
- Full create → list → info → delete workflow passes
- IDs increment correctly (don't reuse)
- Hooks execute in correct order
- JSON output is valid and parseable
- Multiple worktrees work correctly
