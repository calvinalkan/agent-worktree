package main

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

const (
	windowsOSIntegration     = "windows"
	testBaseBranch           = "master"
	testWorktreeNameJSONTest = "json-test"
	testBranchDevelop        = "develop"
)

func Test_FullWorkflow_Create_List_Info_Delete(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOSIntegration {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Step 1: Create first worktree
	createOut, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-a")
	if code != 0 {
		t.Fatalf("create feature-a failed: %s", stderr)
	}

	AssertContains(t, createOut, "name:        feature-a")
	AssertContains(t, createOut, "id:          1")

	wtPathA := extractPath(createOut)

	// Step 2: Create a develop branch and second worktree from it
	cmd := testGitCmd("-C", c.Dir, "branch", testBranchDevelop)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch develop failed: %v\n%s", err, out)
	}

	createOut2, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-b", "--from-branch", testBranchDevelop)
	if code != 0 {
		t.Fatalf("create feature-b failed: %s", stderr)
	}

	AssertContains(t, createOut2, "name:        feature-b")
	AssertContains(t, createOut2, "id:          2")
	AssertContains(t, createOut2, "from:        develop")

	// Step 3: List should show both worktrees
	listOut, stderr, code := c.Run("--config", "config.json", "list")
	if code != 0 {
		t.Fatalf("list failed: %s", stderr)
	}

	AssertContains(t, listOut, "feature-a")
	AssertContains(t, listOut, "feature-b")

	// Step 4: Info from first worktree
	c2 := NewCLITesterAt(t, wtPathA)

	infoOut, stderr, code := c2.Run("--config", "../config.json", "info")
	if code != 0 {
		t.Fatalf("info failed: %s", stderr)
	}

	AssertContains(t, infoOut, "name:        feature-a")
	AssertContains(t, infoOut, "id:          1")
	AssertContains(t, infoOut, "base_branch: master")

	// Step 5: Info JSON format
	infoJSON, stderr, code := c2.Run("--config", "../config.json", "info", "--json")
	if code != 0 {
		t.Fatalf("info --json failed: %s", stderr)
	}

	var info struct {
		Name       string `json:"name"`
		ID         int    `json:"id"`
		BaseBranch string `json:"base_branch"`
	}

	err = json.Unmarshal([]byte(infoJSON), &info)
	if err != nil {
		t.Fatalf("failed to parse info JSON: %v\n%s", err, infoJSON)
	}

	if info.Name != "feature-a" {
		t.Errorf("expected name 'feature-a', got %q", info.Name)
	}

	if info.ID != 1 {
		t.Errorf("expected id 1, got %d", info.ID)
	}

	if info.BaseBranch != testBaseBranch {
		t.Errorf("expected base_branch 'master', got %q", info.BaseBranch)
	}

	// Step 6: Delete first worktree
	deleteOut, stderr, code := c.Run("--config", "config.json", "delete", "feature-a", "--with-branch", "--force")
	if code != 0 {
		t.Fatalf("delete feature-a failed: %s", stderr)
	}

	AssertContains(t, deleteOut, "Deleted worktree and branch: feature-a")

	// Step 7: List should show only second worktree
	listOut, stderr, code = c.Run("--config", "config.json", "list")
	if code != 0 {
		t.Fatalf("list failed: %s", stderr)
	}

	AssertNotContains(t, listOut, "feature-a")
	AssertContains(t, listOut, "feature-b")

	// Step 8: Create new worktree (should get ID 3, not reuse ID 1)
	createOut3, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-c")
	if code != 0 {
		t.Fatalf("create feature-c failed: %s", stderr)
	}

	AssertContains(t, createOut3, "id:          3")

	// Step 9: Delete remaining worktrees
	_, stderr, code = c.Run("--config", "config.json", "delete", "feature-b", "--with-branch", "--force")
	if code != 0 {
		t.Fatalf("delete feature-b failed: %s", stderr)
	}

	_, stderr, code = c.Run("--config", "config.json", "delete", "feature-c", "--with-branch", "--force")
	if code != 0 {
		t.Fatalf("delete feature-c failed: %s", stderr)
	}

	// Step 10: List should be empty (just header or empty)
	listOut, stderr, code = c.Run("--config", "config.json", "list")
	if code != 0 {
		t.Fatalf("list failed: %s", stderr)
	}

	// After deleting all worktrees, list output should not contain any worktree names
	AssertNotContains(t, listOut, "feature-a")
	AssertNotContains(t, listOut, "feature-b")
	AssertNotContains(t, listOut, "feature-c")
}

func Test_Workflow_With_Hooks_Execute_In_Order(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOSIntegration {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Setup hooks that log to a file
	postCreateHook := `#!/bin/bash
echo "CREATED: $WT_NAME" >> "$WT_REPO_ROOT/hook-log.txt"
`
	preDeleteHook := `#!/bin/bash
echo "DELETING: $WT_NAME" >> "$WT_REPO_ROOT/hook-log.txt"
`

	c.WriteExecutable(".wt/hooks/post-create", postCreateHook)
	c.WriteExecutable(".wt/hooks/pre-delete", preDeleteHook)

	// Create worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "hook-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Delete worktree
	_, stderr, code = c.Run("--config", "config.json", "delete", "hook-wt", "--with-branch", "--force")
	if code != 0 {
		t.Fatalf("delete failed: %s", stderr)
	}

	// Verify hooks ran in order
	logContent := c.ReadFile("hook-log.txt")
	lines := strings.Split(strings.TrimSpace(logContent), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "CREATED: hook-wt") {
		t.Errorf("line 1 should be create log, got: %s", lines[0])
	}

	if !strings.Contains(lines[1], "DELETING: hook-wt") {
		t.Errorf("line 2 should be delete log, got: %s", lines[1])
	}
}

func Test_Workflow_List_JSON_Is_Valid_And_Parseable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOSIntegration {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", testWorktreeNameJSONTest)
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Get list as JSON
	listJSON, stderr, code := c.Run("--config", "config.json", "list", "--json")
	if code != 0 {
		t.Fatalf("list --json failed: %s", stderr)
	}

	// Parse JSON
	var worktrees []struct {
		Name       string `json:"name"`
		AgentID    string `json:"agent_id"`
		ID         int    `json:"id"`
		Path       string `json:"path"`
		BaseBranch string `json:"base_branch"`
		Created    string `json:"created"`
	}

	err := json.Unmarshal([]byte(listJSON), &worktrees)
	if err != nil {
		t.Fatalf("failed to parse list JSON: %v\n%s", err, listJSON)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}

	wt := worktrees[0]

	if wt.Name != testWorktreeNameJSONTest {
		t.Errorf("expected name 'json-test', got %q", wt.Name)
	}

	if wt.ID != 1 {
		t.Errorf("expected id 1, got %d", wt.ID)
	}

	if wt.BaseBranch != testBaseBranch {
		t.Errorf("expected base_branch 'master', got %q", wt.BaseBranch)
	}

	if wt.Path == "" {
		t.Error("expected path to be set")
	}

	if wt.Created == "" {
		t.Error("expected created to be set")
	}

	// Verify agent_id is in adjective-animal format
	if !strings.Contains(wt.AgentID, "-") {
		t.Errorf("expected agent_id in adjective-animal format, got %q", wt.AgentID)
	}
}

func Test_Workflow_List_JSON_Parseable_By_Jq(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOSIntegration {
		t.Skip("skipping shell script test on Windows")
	}

	// Skip if jq not available
	_, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq not available")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "jq-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Get list as JSON
	listJSON, stderr, code := c.Run("--config", "config.json", "list", "--json")
	if code != 0 {
		t.Fatalf("list --json failed: %s", stderr)
	}

	// Pipe to jq
	cmd := exec.Command("jq", ".[0].name")
	cmd.Stdin = strings.NewReader(listJSON)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("jq failed: %v", err)
	}

	if strings.TrimSpace(string(out)) != `"jq-test"` {
		t.Errorf("jq output = %s, want \"jq-test\"", string(out))
	}
}

func Test_Workflow_Multiple_Worktrees_With_Different_Base_Branches(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOSIntegration {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create develop and release branches
	cmd := testGitCmd("-C", c.Dir, "branch", testBranchDevelop)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch develop failed: %v\n%s", err, out)
	}

	cmd = testGitCmd("-C", c.Dir, "branch", "release")

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch release failed: %v\n%s", err, out)
	}

	// Create worktrees from different branches
	create1, stderr, code := c.Run("--config", "config.json", "create", "--name", "from-master")
	if code != 0 {
		t.Fatalf("create from-master failed: %s", stderr)
	}

	AssertContains(t, create1, "from:        master")

	create2, stderr, code := c.Run("--config", "config.json", "create", "--name", "from-develop", "--from-branch", testBranchDevelop)
	if code != 0 {
		t.Fatalf("create from-develop failed: %s", stderr)
	}

	AssertContains(t, create2, "from:        develop")

	create3, stderr, code := c.Run("--config", "config.json", "create", "--name", "from-release", "--from-branch", "release")
	if code != 0 {
		t.Fatalf("create from-release failed: %s", stderr)
	}

	AssertContains(t, create3, "from:        release")

	// List and verify all are present
	listOut, stderr, code := c.Run("--config", "config.json", "list")
	if code != 0 {
		t.Fatalf("list failed: %s", stderr)
	}

	AssertContains(t, listOut, "from-master")
	AssertContains(t, listOut, "from-develop")
	AssertContains(t, listOut, "from-release")

	// Verify info shows correct base branches
	wtPath1 := extractPath(create1)
	c1 := NewCLITesterAt(t, wtPath1)

	info1, _, _ := c1.Run("--config", "../config.json", "info", "--field", "base_branch")
	if strings.TrimSpace(info1) != testBaseBranch {
		t.Errorf("from-master base_branch = %q, want 'master'", strings.TrimSpace(info1))
	}

	wtPath2 := extractPath(create2)
	c2 := NewCLITesterAt(t, wtPath2)

	info2, _, _ := c2.Run("--config", "../config.json", "info", "--field", "base_branch")
	if strings.TrimSpace(info2) != testBranchDevelop {
		t.Errorf("from-develop base_branch = %q, want 'develop'", strings.TrimSpace(info2))
	}

	wtPath3 := extractPath(create3)
	c3 := NewCLITesterAt(t, wtPath3)

	info3, _, _ := c3.Run("--config", "../config.json", "info", "--field", "base_branch")
	if strings.TrimSpace(info3) != "release" {
		t.Errorf("from-release base_branch = %q, want 'release'", strings.TrimSpace(info3))
	}
}
