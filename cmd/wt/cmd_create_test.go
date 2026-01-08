package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

func Test_Create_Returns_Error_When_Not_In_Git_Repo(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	// Don't initialize git repo

	_, stderr, code := cli.Run("create")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a git repository")
}

func Test_Create_Creates_Worktree_With_Defaults(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Check output format
	AssertContains(t, stdout, "Created worktree:")
	AssertContains(t, stdout, "name:")
	AssertContains(t, stdout, "agent_id:")
	AssertContains(t, stdout, "id:")
	AssertContains(t, stdout, "path:")
	AssertContains(t, stdout, "branch:")
	AssertContains(t, stdout, "from:")

	// ID should be 1 for first worktree
	AssertContains(t, stdout, "id:          1")

	// from should be main (default branch)
	AssertContains(t, stdout, "from:        master")
}

func Test_Create_Creates_Worktree_Directory_And_Metadata(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	wtBaseDir := filepath.Join(cli.Dir, "worktrees")
	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Extract name from output (name and agent_id are same for default)
	// Output format: "  name:        swift-fox"
	var name string

	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, "name:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name = parts[1]

				break
			}
		}
	}

	if name == "" {
		t.Fatal("could not extract worktree name from output")
	}

	// Verify worktree directory was created
	wtPath := filepath.Join(wtBaseDir, name)

	if !cli.FileExists(filepath.Join("worktrees", name)) {
		t.Errorf("worktree directory %s was not created", wtPath)
	}

	// Verify .wt/worktree.json was created
	metadataPath := filepath.Join("worktrees", name, ".wt", "worktree.json")
	if !cli.FileExists(metadataPath) {
		t.Error("worktree metadata file was not created")
	}

	// Verify metadata content
	fsys := fs.NewReal()

	info, err := readWorktreeInfo(fsys, wtPath)
	if err != nil {
		t.Fatalf("failed to read worktree info: %v", err)
	}

	if info.Name != name {
		t.Errorf("expected name %q, got %q", name, info.Name)
	}

	if info.AgentID != name {
		t.Errorf("expected agent_id %q, got %q", name, info.AgentID)
	}

	if info.ID != 1 {
		t.Errorf("expected id 1, got %d", info.ID)
	}

	if info.BaseBranch != testBaseBranchMain {
		t.Errorf("expected base_branch 'master', got %q", info.BaseBranch)
	}
}

func Test_Create_With_Custom_Name(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "my-feature")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Name should be the custom name
	AssertContains(t, stdout, "name:        my-feature")
	AssertContains(t, stdout, "branch:      my-feature")

	// agent_id should still be auto-generated (different from name)
	// We verify name != agent_id by checking they're on different lines with different values
	lines := strings.Split(stdout, "\n")

	var foundName, foundAgentID string

	for _, line := range lines {
		if strings.Contains(line, "name:") && !strings.Contains(line, "agent") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				foundName = parts[1]
			}
		}

		if strings.Contains(line, "agent_id:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				foundAgentID = parts[1]
			}
		}
	}

	if foundName != "my-feature" {
		t.Errorf("expected name 'my-feature', got %q", foundName)
	}

	if foundAgentID == "" {
		t.Error("agent_id was not generated")
	}

	// Verify worktree directory uses custom name
	if !cli.FileExists(filepath.Join("worktrees", "my-feature")) {
		t.Error("worktree directory 'my-feature' was not created")
	}
}

func Test_Create_From_Specific_Branch(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a develop branch
	createBranch(t, cli.Dir, "develop")

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--from-branch", "develop")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "from:        develop")
}

func Test_Create_Increments_ID(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create first worktree
	stdout1, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-one")
	if code != 0 {
		t.Fatalf("first create failed: %s", stderr)
	}

	AssertContains(t, stdout1, "id:          1")

	// Create second worktree
	stdout2, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-two")
	if code != 0 {
		t.Fatalf("second create failed: %s", stderr)
	}

	AssertContains(t, stdout2, "id:          2")

	// Create third worktree
	stdout3, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-three")
	if code != 0 {
		t.Fatalf("third create failed: %s", stderr)
	}

	AssertContains(t, stdout3, "id:          3")
}

func Test_Create_Returns_Error_When_Name_Already_In_Use(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create first worktree with custom name
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "duplicate-name")
	if code != 0 {
		t.Fatalf("first create failed: %s", stderr)
	}

	// Try to create second worktree with same name
	_, stderr, code = cli.Run("--config", "config.json", "create", "--name", "duplicate-name")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "already in use")
}

func Test_Create_Returns_Error_When_Branch_Already_Exists(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a branch manually
	createBranch(t, cli.Dir, "existing-branch")

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Try to create worktree with same name as existing branch
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "existing-branch")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "creating worktree")
}

func Test_Create_Runs_Post_Create_Hook(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a post-create hook that writes a marker file
	hookScript := `#!/bin/bash
echo "hook ran with WT_NAME=$WT_NAME" > "$WT_PATH/hook-marker.txt"
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "hook-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")

	// Verify hook ran by checking marker file
	markerPath := filepath.Join("worktrees", "hook-test", "hook-marker.txt")
	if !cli.FileExists(markerPath) {
		t.Error("hook marker file was not created - hook did not run")
	}

	markerContent := cli.ReadFile(markerPath)
	AssertContains(t, markerContent, "hook ran with WT_NAME=hook-test")
}

func Test_Create_Rollback_On_Hook_Failure(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a post-create hook that fails
	hookScript := `#!/bin/bash
exit 1
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "failing-hook")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "post-create hook failed")

	// Verify worktree was rolled back (directory should not exist)
	if cli.FileExists(filepath.Join("worktrees", "failing-hook")) {
		t.Error("worktree directory should have been removed on hook failure")
	}

	// Verify branch was deleted
	branches := listBranches(t, cli.Dir)
	for _, branch := range branches {
		if branch == "failing-hook" {
			t.Error("branch 'failing-hook' should have been deleted on rollback")
		}
	}
}

func Test_Create_Returns_Error_When_Hook_Not_Executable(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a post-create hook that is NOT executable
	hookScript := `#!/bin/bash
echo "should not run"
`
	cli.WriteFile(".wt/hooks/post-create", hookScript)
	// Don't chmod +x

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "non-exec-hook")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not executable")
}

func Test_Create_Hook_Receives_Environment_Variables(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a hook that dumps all WT_* environment variables
	hookScript := `#!/bin/bash
{
    echo "WT_ID=$WT_ID"
    echo "WT_AGENT_ID=$WT_AGENT_ID"
    echo "WT_NAME=$WT_NAME"
    echo "WT_PATH=$WT_PATH"
    echo "WT_BASE_BRANCH=$WT_BASE_BRANCH"
    echo "WT_REPO_ROOT=$WT_REPO_ROOT"
    echo "WT_SOURCE=$WT_SOURCE"
} > "$WT_PATH/env-dump.txt"
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "env-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Read the env dump
	envContent := cli.ReadFile(filepath.Join("worktrees", "env-test", "env-dump.txt"))

	// Check exact values
	AssertContains(t, envContent, "WT_ID=1")
	AssertContains(t, envContent, "WT_NAME=env-test")
	AssertContains(t, envContent, "WT_BASE_BRANCH=master")
	AssertContains(t, envContent, "WT_REPO_ROOT="+cli.Dir)
	AssertContains(t, envContent, "WT_SOURCE="+cli.Dir)
	AssertContains(t, envContent, "WT_PATH="+filepath.Join(cli.Dir, "worktrees", "env-test"))

	// WT_AGENT_ID should be an adjective-animal format (contains a hyphen)
	// We can't predict the exact value, but verify it's set and non-empty
	lines := strings.Split(envContent, "\n")

	var agentID string

	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, "WT_AGENT_ID="); ok {
			agentID = after

			break
		}
	}

	if agentID == "" {
		t.Error("WT_AGENT_ID was not set")
	}

	if !strings.Contains(agentID, "-") {
		t.Errorf("WT_AGENT_ID should be adjective-animal format, got: %q", agentID)
	}
}

func Test_Create_Uses_Short_Name_Flag(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "-n", "short-flag")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        short-flag")
}

func Test_Create_Uses_Short_From_Branch_Flag(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	createBranch(t, cli.Dir, "feature")

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "-b", "feature")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "from:        feature")
}

func Test_Create_With_Changes_Flag_Is_Accepted(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// The flag should be accepted (even if not fully implemented yet)
	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "with-changes-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")
}

func Test_Create_Worktree_Is_Valid_Git_Worktree(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "valid-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	AssertContains(t, stdout, "Created worktree:")

	// Verify it's a valid git worktree by running git status in it
	wtPath := filepath.Join(cli.Dir, "worktrees", "valid-wt")

	git := NewGit(filterTestGitEnv(os.Environ()))

	_, err := git.CurrentBranch(context.Background(), wtPath)
	if err != nil {
		t.Errorf("worktree is not a valid git repository: %v", err)
	}

	// Verify the branch name matches
	branch, err := git.CurrentBranch(context.Background(), wtPath)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}

	if branch != "valid-wt" {
		t.Errorf("expected branch 'valid-wt', got %q", branch)
	}
}

func Test_Create_Metadata_Has_Valid_Timestamp(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "timestamp-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Read and parse the metadata
	wtPath := filepath.Join(cli.Dir, "worktrees", "timestamp-test")
	fsys := fs.NewReal()

	info, err := readWorktreeInfo(fsys, wtPath)
	if err != nil {
		t.Fatalf("failed to read worktree info: %v", err)
	}

	// Created timestamp should be recent (within last minute)
	if info.Created.IsZero() {
		t.Error("created timestamp is zero")
	}

	// Verify JSON format has ISO 8601
	jsonPath := filepath.Join(wtPath, ".wt", "worktree.json")

	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read worktree.json: %v", err)
	}

	// Should contain Z suffix for UTC
	if !strings.Contains(string(content), "Z\"") {
		t.Error("timestamp should be in UTC (end with Z)")
	}
}

func Test_Create_Multiple_Worktrees_Have_Unique_Agent_IDs(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	agentIDs := make(map[string]bool)

	// Create multiple worktrees and collect their agent_ids
	for i := range 5 {
		stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-"+string(rune('a'+i)))
		if code != 0 {
			t.Fatalf("create %d failed: %s", i, stderr)
		}

		// Extract agent_id from output
		for line := range strings.SplitSeq(stdout, "\n") {
			if strings.Contains(line, "agent_id:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					agentID := parts[1]
					if agentIDs[agentID] {
						t.Errorf("duplicate agent_id: %s", agentID)
					}

					agentIDs[agentID] = true
				}
			}
		}
	}

	if len(agentIDs) != 5 {
		t.Errorf("expected 5 unique agent_ids, got %d", len(agentIDs))
	}
}

func Test_Create_Help_Shows_Usage(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	stdout, _, code := cli.Run("create", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}

	AssertContains(t, stdout, "Usage: wt create")
	AssertContains(t, stdout, "--name")
	AssertContains(t, stdout, "--from-branch")
	AssertContains(t, stdout, "--with-changes")
}

func Test_Create_Help_Shows_Detailed_Description(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)

	stdout, _, code := cli.Run("create", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}

	// Verify description explains what gets created
	AssertContains(t, stdout, "A git branch is created with the same name as the worktree")
	AssertContains(t, stdout, "<base>/<repo>/<name>")
	AssertContains(t, stdout, ".wt/worktree.json")
	AssertContains(t, stdout, "post-create")

	// Verify improved flag descriptions
	AssertContains(t, stdout, "Worktree and branch name (default: auto-generated)")
	AssertContains(t, stdout, "Branch to base off (default: current branch)")
	AssertContains(t, stdout, "Copy staged, unstaged, and untracked files")
}

func Test_Create_Output_Path_Is_Absolute(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "abs-path")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Extract path from output
	var foundPath string

	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, "path:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				foundPath = parts[1]

				break
			}
		}
	}

	if foundPath == "" {
		t.Fatal("could not extract path from output")
	}

	if !filepath.IsAbs(foundPath) {
		t.Errorf("path should be absolute, got: %s", foundPath)
	}
}

func Test_Create_JSON_Metadata_Is_Valid(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "json-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Read and parse the JSON metadata
	jsonPath := filepath.Join(cli.Dir, "worktrees", "json-test", ".wt", "worktree.json")

	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read worktree.json: %v", err)
	}

	var metadata map[string]any

	err = json.Unmarshal(content, &metadata)
	if err != nil {
		t.Fatalf("worktree.json is not valid JSON: %v", err)
	}

	// Verify required fields exist
	requiredFields := []string{"name", "agent_id", "id", "base_branch", "created"}
	for _, field := range requiredFields {
		if _, ok := metadata[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify field values
	if metadata["name"] != "json-test" {
		t.Errorf("expected name 'json-test', got %v", metadata["name"])
	}

	if metadata["base_branch"] != testBaseBranchMain {
		t.Errorf("expected base_branch 'master', got %v", metadata["base_branch"])
	}

	// id should be a number
	if id, ok := metadata["id"].(float64); !ok || id != 1 {
		t.Errorf("expected id 1, got %v", metadata["id"])
	}
}

// Tests for --with-changes flag

func Test_Create_With_Changes_Copies_Modified_Tracked_File(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Modify an existing tracked file
	cli.WriteFile("README.md", "# Modified content\n")

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-modified")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")

	// Verify the modified file was copied
	copiedContent := cli.ReadFile(filepath.Join("worktrees", "wt-modified", "README.md"))
	if copiedContent != "# Modified content\n" {
		t.Errorf("expected modified content, got: %q", copiedContent)
	}
}

func Test_Create_With_Changes_Copies_Untracked_File(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a new untracked file
	cli.WriteFile("new-file.txt", "new untracked content\n")

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-untracked")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")

	// Verify the untracked file was copied
	copiedContent := cli.ReadFile(filepath.Join("worktrees", "wt-untracked", "new-file.txt"))
	if copiedContent != "new untracked content\n" {
		t.Errorf("expected untracked content, got: %q", copiedContent)
	}
}

func Test_Create_With_Changes_Copies_Staged_File(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create and stage a new file
	cli.WriteFile("staged-file.txt", "staged content\n")

	cmd := testGitCmd("add", "staged-file.txt")
	cmd.Dir = cli.Dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to stage file: %v\n%s", err, out)
	}

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-staged")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")

	// Verify the staged file was copied
	copiedContent := cli.ReadFile(filepath.Join("worktrees", "wt-staged", "staged-file.txt"))
	if copiedContent != "staged content\n" {
		t.Errorf("expected staged content, got: %q", copiedContent)
	}
}

func Test_Create_With_Changes_Respects_Gitignore(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a .gitignore and commit it
	cli.WriteFile(".gitignore", "*.log\nignored-dir/\n")

	cmd := testGitCmd("add", ".gitignore")
	cmd.Dir = cli.Dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to add .gitignore: %v\n%s", err, out)
	}

	cmd = testGitCmd("commit", "-m", "add gitignore")
	cmd.Dir = cli.Dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to commit .gitignore: %v\n%s", err, out)
	}

	// Create ignored files
	cli.WriteFile("debug.log", "debug log content\n")
	cli.WriteFile("ignored-dir/secret.txt", "secret content\n")

	// Create a non-ignored untracked file
	cli.WriteFile("should-copy.txt", "this should be copied\n")

	_, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-gitignore")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify ignored files were NOT copied
	if cli.FileExists(filepath.Join("worktrees", "wt-gitignore", "debug.log")) {
		t.Error("debug.log should not have been copied (gitignored)")
	}

	if cli.FileExists(filepath.Join("worktrees", "wt-gitignore", "ignored-dir", "secret.txt")) {
		t.Error("ignored-dir/secret.txt should not have been copied (gitignored)")
	}

	// Verify non-ignored file WAS copied
	copiedContent := cli.ReadFile(filepath.Join("worktrees", "wt-gitignore", "should-copy.txt"))
	if copiedContent != "this should be copied\n" {
		t.Errorf("expected non-ignored content, got: %q", copiedContent)
	}
}

func Test_Create_With_Changes_Copies_Nested_Directory_Structure(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create nested directory structure with untracked files
	cli.WriteFile("src/pkg/main.go", "package main\n")
	cli.WriteFile("src/pkg/util/helper.go", "package util\n")
	cli.WriteFile("docs/readme.md", "# Docs\n")

	_, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-nested")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify all nested files were copied with correct structure
	content1 := cli.ReadFile(filepath.Join("worktrees", "wt-nested", "src", "pkg", "main.go"))
	if content1 != "package main\n" {
		t.Errorf("expected main.go content, got: %q", content1)
	}

	content2 := cli.ReadFile(filepath.Join("worktrees", "wt-nested", "src", "pkg", "util", "helper.go"))
	if content2 != "package util\n" {
		t.Errorf("expected helper.go content, got: %q", content2)
	}

	content3 := cli.ReadFile(filepath.Join("worktrees", "wt-nested", "docs", "readme.md"))
	if content3 != "# Docs\n" {
		t.Errorf("expected readme.md content, got: %q", content3)
	}
}

func Test_Create_With_Changes_Copies_Both_Staged_And_Unstaged(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create and stage a file
	cli.WriteFile("staged.txt", "staged\n")

	cmd := testGitCmd("add", "staged.txt")
	cmd.Dir = cli.Dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to stage file: %v\n%s", err, out)
	}

	// Create an unstaged modification to tracked file
	cli.WriteFile("README.md", "unstaged modification\n")

	// Create an untracked file
	cli.WriteFile("untracked.txt", "untracked\n")

	_, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-all")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify all three types were copied
	if cli.ReadFile(filepath.Join("worktrees", "wt-all", "staged.txt")) != "staged\n" {
		t.Error("staged file was not copied correctly")
	}

	if cli.ReadFile(filepath.Join("worktrees", "wt-all", "README.md")) != "unstaged modification\n" {
		t.Error("unstaged modification was not copied correctly")
	}

	if cli.ReadFile(filepath.Join("worktrees", "wt-all", "untracked.txt")) != "untracked\n" {
		t.Error("untracked file was not copied correctly")
	}
}

func Test_Create_Without_Changes_Does_Not_Copy_Uncommitted(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create an untracked file
	cli.WriteFile("local-only.txt", "local content\n")

	// Create worktree WITHOUT --with-changes
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-clean")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify the untracked file was NOT copied
	if cli.FileExists(filepath.Join("worktrees", "wt-clean", "local-only.txt")) {
		t.Error("untracked file should not have been copied without --with-changes")
	}
}

func Test_Create_With_Changes_No_Changes_Succeeds(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create worktree with --with-changes when there are no changes
	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--with-changes", "--name", "wt-no-changes")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Created worktree:")
}

// Tests for rollback error handling with errors.Join

func Test_Create_Rollback_On_Hook_Failure_Shows_Only_Hook_Error_When_Rollback_Succeeds(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a post-create hook that fails
	hookScript := `#!/bin/bash
echo "hook failed deliberately" >&2
exit 1
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "rollback-test")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	// Should see hook error
	AssertContains(t, stderr, "post-create hook failed")

	// Should NOT see rollback errors (since rollback succeeded)
	if strings.Contains(stderr, "git worktree remove failed") {
		t.Error("should not see worktree remove error when rollback succeeded")
	}

	if strings.Contains(stderr, "git branch delete failed") {
		t.Error("should not see branch delete error when rollback succeeded")
	}
}

func Test_Create_Rollback_On_Hook_Failure_Includes_Rollback_Errors_When_Rollback_Fails(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a post-create hook that:
	// 1. Makes .git/worktrees unreadable so git worktree remove will fail
	// 2. Exits with error
	hookScript := `#!/bin/bash
chmod 000 "$WT_REPO_ROOT/.git/worktrees"
exit 1
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "bad-rollback")

	// Restore permissions so cleanup can happen
	_ = os.Chmod(filepath.Join(cli.Dir, ".git", "worktrees"), 0o700)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	// Should see hook error
	AssertContains(t, stderr, "post-create hook failed")

	// Should see rollback error because git couldn't access .git/worktrees
	AssertContains(t, stderr, "removing worktree")
}

func Test_Create_Rollback_Includes_Both_Errors_When_Both_Operations_Fail(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a hook that:
	// 1. Makes .git/worktrees unreadable (worktree remove fails)
	// 2. Makes .git/refs/heads unreadable (branch delete fails)
	// 3. Exits with error
	hookScript := `#!/bin/bash
chmod 000 "$WT_REPO_ROOT/.git/worktrees"
chmod 000 "$WT_REPO_ROOT/.git/refs/heads"
exit 1
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "both-fail")

	// Restore permissions so cleanup can happen
	_ = os.Chmod(filepath.Join(cli.Dir, ".git", "worktrees"), 0o700)
	_ = os.Chmod(filepath.Join(cli.Dir, ".git", "refs", "heads"), 0o700)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	// Should see hook error
	AssertContains(t, stderr, "post-create hook failed")

	// Should see worktree remove error
	AssertContains(t, stderr, "removing worktree")

	// Should see branch delete error
	AssertContains(t, stderr, "deleting branch")
}

// Tests for concurrent worktree creation

func Test_Create_Concurrent_Creates_Have_Unique_IDs(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	const numWorktrees = 20

	type result struct {
		id     int
		name   string
		stderr string
		code   int
	}

	results := make(chan result, numWorktrees)

	// Launch all goroutines simultaneously using a barrier
	var wg sync.WaitGroup

	start := make(chan struct{})

	for i := range numWorktrees {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			// Wait for start signal (barrier)
			<-start

			name := fmt.Sprintf("stress-wt-%d", idx)
			stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", name)

			var id int

			for line := range strings.SplitSeq(stdout, "\n") {
				if strings.Contains(line, "id:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						_, _ = fmt.Sscanf(fields[1], "%d", &id)
					}
				}
			}

			results <- result{id: id, name: name, stderr: stderr, code: code}
		}(i)
	}

	// Release all goroutines at once
	close(start)

	// Wait for all to complete
	wg.Wait()
	close(results)

	// Collect and verify results
	ids := make(map[int]string)
	successCount := 0

	for r := range results {
		if r.code != 0 {
			t.Errorf("create %s failed: %s", r.name, r.stderr)

			continue
		}

		successCount++

		if r.id == 0 {
			t.Errorf("could not extract ID for %s", r.name)

			continue
		}

		if existingName, exists := ids[r.id]; exists {
			t.Errorf("RACE CONDITION: duplicate ID %d assigned to both %s and %s", r.id, existingName, r.name)
		}

		ids[r.id] = r.name
	}

	// All should succeed
	if successCount != numWorktrees {
		t.Errorf("expected %d successful creates, got %d", numWorktrees, successCount)
	}

	// All IDs should be unique and sequential
	if len(ids) != numWorktrees {
		t.Errorf("expected %d unique IDs, got %d", numWorktrees, len(ids))
	}

	for i := 1; i <= numWorktrees; i++ {
		if _, exists := ids[i]; !exists {
			t.Errorf("missing expected ID %d in sequence", i)
		}
	}
}

func Test_Create_Concurrent_From_Multiple_Worktrees(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// First create two worktrees sequentially
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-a")
	if code != 0 {
		t.Fatalf("create wt-a failed: %s", stderr)
	}

	_, stderr, code = cli.Run("--config", "config.json", "create", "--name", "wt-b")
	if code != 0 {
		t.Fatalf("create wt-b failed: %s", stderr)
	}

	// Copy config to both worktrees
	configContent := cli.ReadFile("config.json")
	cli.WriteFile(filepath.Join("worktrees", "wt-a", "config.json"), configContent)
	cli.WriteFile(filepath.Join("worktrees", "wt-b", "config.json"), configContent)

	wtPathA := filepath.Join(cli.Dir, "worktrees", "wt-a")
	wtPathB := filepath.Join(cli.Dir, "worktrees", "wt-b")

	const numPerWorktree = 5

	type result struct {
		id       int
		name     string
		stderr   string
		code     int
		fromPath string
	}

	results := make(chan result, numPerWorktree*2)

	var wg sync.WaitGroup

	start := make(chan struct{})

	// Launch creates from worktree A
	for i := range numPerWorktree {
		wg.Add(1)

		go func(idx int, wtPath string) {
			defer wg.Done()

			<-start

			name := fmt.Sprintf("from-a-%d", idx)
			stdout, stderr, code := cli.RunInDir(wtPath, "--config", "config.json", "create", "--name", name)

			var id int

			for line := range strings.SplitSeq(stdout, "\n") {
				if strings.Contains(line, "id:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						_, _ = fmt.Sscanf(fields[1], "%d", &id)
					}
				}
			}

			results <- result{id: id, name: name, stderr: stderr, code: code, fromPath: wtPath}
		}(i, wtPathA)
	}

	// Launch creates from worktree B
	for i := range numPerWorktree {
		wg.Add(1)

		go func(idx int, wtPath string) {
			defer wg.Done()

			<-start

			name := fmt.Sprintf("from-b-%d", idx)
			stdout, stderr, code := cli.RunInDir(wtPath, "--config", "config.json", "create", "--name", name)

			var id int

			for line := range strings.SplitSeq(stdout, "\n") {
				if strings.Contains(line, "id:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						_, _ = fmt.Sscanf(fields[1], "%d", &id)
					}
				}
			}

			results <- result{id: id, name: name, stderr: stderr, code: code, fromPath: wtPath}
		}(i, wtPathB)
	}

	// Release all goroutines
	close(start)

	wg.Wait()
	close(results)

	// Verify results
	ids := make(map[int]string)
	successCount := 0

	for r := range results {
		if r.code != 0 {
			t.Errorf("create %s from %s failed: %s", r.name, r.fromPath, r.stderr)

			continue
		}

		successCount++

		if r.id == 0 {
			t.Errorf("could not extract ID for %s", r.name)

			continue
		}

		if existingName, exists := ids[r.id]; exists {
			t.Errorf("RACE CONDITION: duplicate ID %d assigned to both %s and %s", r.id, existingName, r.name)
		}

		ids[r.id] = r.name
	}

	expectedTotal := numPerWorktree * 2

	if successCount != expectedTotal {
		t.Errorf("expected %d successful creates, got %d", expectedTotal, successCount)
	}

	// IDs should start from 3 (1=wt-a, 2=wt-b) and go up
	expectedIDs := make(map[int]bool)
	for i := 3; i <= 2+expectedTotal; i++ {
		expectedIDs[i] = true
	}

	for id := range ids {
		if !expectedIDs[id] {
			t.Errorf("unexpected ID %d (expected 3-%d)", id, 2+expectedTotal)
		}

		delete(expectedIDs, id)
	}

	for id := range expectedIDs {
		t.Errorf("missing expected ID %d", id)
	}
}

func Test_Create_Lock_File_Is_Inside_Git_Directory(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "lock-test")

	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Lock file should be inside .git directory
	if !cli.FileExists(".git/wt.lock") {
		t.Error(".git/wt.lock should exist after create")
	}

	// No orphan lock file should be created in base directory
	if cli.FileExists(filepath.Join("worktrees", ".wt-create.lock")) {
		t.Error("orphan lock file should not be created in worktrees directory")
	}
}

func Test_Create_From_Worktree_Uses_Shared_Lock(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create first worktree from main repo
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "wt-first")
	if code != 0 {
		t.Fatalf("first create failed: %s", stderr)
	}

	// Verify lock file is in main repo's .git
	if !cli.FileExists(".git/wt.lock") {
		t.Error(".git/wt.lock should exist")
	}

	// Now create second worktree from inside the first worktree
	wtPath := filepath.Join(cli.Dir, "worktrees", "wt-first")

	// Copy config to worktree
	configContent := cli.ReadFile("config.json")
	cli.WriteFile(filepath.Join("worktrees", "wt-first", "config.json"), configContent)

	// Run create from inside the worktree
	stdout, stderr, code := cli.RunInDir(wtPath, "--config", "config.json", "create", "--name", "wt-second")
	if code != 0 {
		t.Fatalf("second create from worktree failed: %s", stderr)
	}

	// Should have unique ID (not 1, since wt-first is 1)
	AssertContains(t, stdout, "id:          2")

	// Lock file should still be in main repo's .git (not in worktree)
	if !cli.FileExists(".git/wt.lock") {
		t.Error("lock file should remain in main .git directory")
	}
}

// Tests for early lock release

func Test_Create_Lock_Released_After_Metadata_Written(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a hook that verifies the lock file is NOT held by checking
	// if we can acquire it ourselves with flock.
	// If lock is still held, flock with LOCK_NB will fail immediately.
	hookScript := `#!/bin/bash
# Try to acquire lock with non-blocking mode
# If lock is already held, this will fail immediately
exec 200>"$WT_REPO_ROOT/.git/wt.lock"
if flock -n 200; then
    echo "LOCK_FREE" > "$WT_PATH/lock-status.txt"
    flock -u 200
else
    echo "LOCK_HELD" > "$WT_PATH/lock-status.txt"
fi
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "lock-check")

	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Read the lock status written by the hook
	lockStatus := strings.TrimSpace(cli.ReadFile(filepath.Join("worktrees", "lock-check", "lock-status.txt")))
	if lockStatus != "LOCK_FREE" {
		t.Errorf("lock should be released before hook runs, but hook reported: %s", lockStatus)
	}
}

func Test_Create_Concurrent_Create_During_Hook_Execution(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a hook that sleeps for 2 seconds and records its start time
	hookScript := `#!/bin/bash
echo "hook started for $WT_NAME at $(date +%s)" >> "$WT_REPO_ROOT/hook-log.txt"
sleep 2
echo "hook finished for $WT_NAME at $(date +%s)" >> "$WT_REPO_ROOT/hook-log.txt"
`
	cli.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Use separate goroutine and wait for both to complete
	type result struct {
		name   string
		stdout string
		stderr string
		code   int
	}

	results := make(chan result, 2)

	// Start two creates concurrently, with the second starting during the first hook
	go func() {
		stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "first-wt")
		results <- result{name: "first", stdout: stdout, stderr: stderr, code: code}
	}()

	// Wait a bit for first create to start its hook (after lock is released)
	time.Sleep(500 * time.Millisecond)

	go func() {
		stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "second-wt")
		results <- result{name: "second", stdout: stdout, stderr: stderr, code: code}
	}()

	// Wait for both to complete
	r1 := <-results
	r2 := <-results

	// Both should succeed
	if r1.code != 0 {
		t.Errorf("%s create failed: %s", r1.name, r1.stderr)
	}

	if r2.code != 0 {
		t.Errorf("%s create failed: %s", r2.name, r2.stderr)
	}

	// Verify worktrees exist
	if !cli.FileExists(filepath.Join("worktrees", "first-wt")) {
		t.Error("first-wt worktree should exist")
	}

	if !cli.FileExists(filepath.Join("worktrees", "second-wt")) {
		t.Error("second-wt worktree should exist")
	}

	// Check that hooks ran concurrently by examining the log
	hookLog := cli.ReadFile("hook-log.txt")

	// Both hooks should have started (4 lines: 2 start + 2 finish)
	lines := strings.Split(strings.TrimSpace(hookLog), "\n")
	if len(lines) < 4 {
		t.Logf("hook log:\n%s", hookLog)
		t.Errorf("expected at least 4 hook log lines, got %d", len(lines))
	}
}

func Test_Create_Adds_Worktree_Exclusion_To_Git_Exclude(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "exclude-test")

	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Verify .wt/worktree.json is in .git/info/exclude
	excludeContent := cli.ReadFile(".git/info/exclude")
	AssertContains(t, excludeContent, ".wt/worktree.json")
}

func Test_Create_Does_Not_Duplicate_Worktree_Exclusion(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create first worktree
	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "first-excl")
	if code != 0 {
		t.Fatalf("first create failed: %s", stderr)
	}

	// Create second worktree
	_, stderr, code = cli.Run("--config", "config.json", "create", "--name", "second-excl")
	if code != 0 {
		t.Fatalf("second create failed: %s", stderr)
	}

	// Read exclude file and count occurrences
	excludeContent := cli.ReadFile(".git/info/exclude")
	count := strings.Count(excludeContent, ".wt/worktree.json")

	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of .wt/worktree.json in exclude, got %d\ncontent:\n%s", count, excludeContent)
	}
}

func Test_Create_Preserves_Existing_Exclude_Content(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Write some existing content to .git/info/exclude
	existingContent := "# Custom exclusions\n*.log\ntmp/\n"
	cli.WriteFile(".git/info/exclude", existingContent)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "preserve-test")

	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Verify existing content is preserved
	excludeContent := cli.ReadFile(".git/info/exclude")
	AssertContains(t, excludeContent, "# Custom exclusions")
	AssertContains(t, excludeContent, "*.log")
	AssertContains(t, excludeContent, "tmp/")
	AssertContains(t, excludeContent, ".wt/worktree.json")
}

func Test_Create_Handles_Exclude_Already_Present(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Pre-add the exclusion manually
	cli.WriteFile(".git/info/exclude", "# Already has it\n.wt/worktree.json\n")

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--name", "already-present")

	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Should still have exactly one occurrence
	excludeContent := cli.ReadFile(".git/info/exclude")
	count := strings.Count(excludeContent, ".wt/worktree.json")

	if count != 1 {
		t.Errorf("expected exactly 1 occurrence, got %d\ncontent:\n%s", count, excludeContent)
	}
}

func Test_Create_Warns_When_Exclude_File_Not_Writable(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Make .git/info/exclude non-writable
	excludePath := filepath.Join(cli.Dir, ".git", "info", "exclude")

	err := os.Chmod(excludePath, 0o444)
	if err != nil {
		t.Fatalf("failed to make exclude read-only: %v", err)
	}

	// Restore permissions after test
	t.Cleanup(func() {
		_ = os.Chmod(excludePath, 0o644)
	})

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "warn-test")

	// Should still succeed (warning is non-blocking)
	if code != 0 {
		t.Fatalf("create should succeed despite exclude warning: %s", stderr)
	}

	// Should print warning to stderr
	AssertContains(t, stderr, "warning:")
	AssertContains(t, stderr, ".wt/worktree.json")
	AssertContains(t, stderr, "manually")

	// Worktree should still be created
	AssertContains(t, stdout, "Created worktree:")
}

func Test_Create_Warns_When_Exclude_File_Not_Readable(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Make .git/info/exclude non-readable
	excludePath := filepath.Join(cli.Dir, ".git", "info", "exclude")

	err := os.Chmod(excludePath, 0o000)
	if err != nil {
		t.Fatalf("failed to make exclude unreadable: %v", err)
	}

	// Restore permissions after test
	t.Cleanup(func() {
		_ = os.Chmod(excludePath, 0o644)
	})

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--name", "unreadable-test")

	// Should still succeed (warning is non-blocking)
	if code != 0 {
		t.Fatalf("create should succeed despite exclude warning: %s", stderr)
	}

	// Should print warning to stderr
	AssertContains(t, stderr, "warning:")
	AssertContains(t, stderr, "manually")

	// Worktree should still be created
	AssertContains(t, stdout, "Created worktree:")
}

// Tests for --json flag

func Test_Create_JSON_Flag_Outputs_Valid_JSON(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "json-out")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Parse the JSON output
	var result map[string]any

	err := json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	// Verify required fields exist
	requiredFields := []string{"name", "agent_id", "id", "path", "branch", "from"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func Test_Create_JSON_Output_Contains_Correct_Values(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "json-values")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	var result map[string]any

	err := json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}

	// Verify specific values
	if result["name"] != "json-values" {
		t.Errorf("expected name 'json-values', got %v", result["name"])
	}

	if result["branch"] != "json-values" {
		t.Errorf("expected branch 'json-values', got %v", result["branch"])
	}

	if result["from"] != testBaseBranchMain {
		t.Errorf("expected from '%s', got %v", testBaseBranchMain, result["from"])
	}

	// id should be 1 for first worktree
	if id, ok := result["id"].(float64); !ok || id != 1 {
		t.Errorf("expected id 1, got %v", result["id"])
	}

	// agent_id should be a string containing hyphen (adjective-animal)
	agentID, ok := result["agent_id"].(string)
	if !ok || !strings.Contains(agentID, "-") {
		t.Errorf("expected agent_id to be adjective-animal format, got %v", result["agent_id"])
	}

	// path should be absolute
	path, ok := result["path"].(string)
	if !ok || !filepath.IsAbs(path) {
		t.Errorf("expected path to be absolute, got %v", result["path"])
	}
}

func Test_Create_JSON_Output_Has_Indentation(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "json-indent")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// JSON should have indentation (contains newlines and spaces)
	if !strings.Contains(stdout, "\n  ") {
		t.Errorf("JSON output should be indented, got:\n%s", stdout)
	}
}

func Test_Create_JSON_Flag_Does_Not_Include_Created_Worktree_Header(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "json-no-header")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should NOT contain the human-readable header
	if strings.Contains(stdout, "Created worktree:") {
		t.Errorf("JSON output should not contain human-readable header")
	}
}

func Test_Create_JSON_Flag_Warnings_Go_To_Stderr(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Make .git/info/exclude non-writable to trigger a warning
	excludePath := filepath.Join(cli.Dir, ".git", "info", "exclude")

	err := os.Chmod(excludePath, 0o444)
	if err != nil {
		t.Fatalf("failed to make exclude read-only: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(excludePath, 0o644)
	})

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "json-warnings")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Warning should be in stderr, not stdout
	AssertContains(t, stderr, "warning:")

	// stdout should still be valid JSON (warning not mixed in)
	var result map[string]any

	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		t.Fatalf("stdout should be valid JSON even with warnings: %v\nstdout: %s", err, stdout)
	}
}

func Test_Create_JSON_Flag_With_Custom_Name(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--name", "custom-json-name")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	var result map[string]any

	err := json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}

	// name should be the custom name
	if result["name"] != "custom-json-name" {
		t.Errorf("expected name 'custom-json-name', got %v", result["name"])
	}

	// agent_id should be different (auto-generated)
	agentID, ok := result["agent_id"].(string)
	if !ok {
		t.Errorf("agent_id should be a string, got %v", result["agent_id"])
	}

	if agentID == "custom-json-name" {
		t.Errorf("agent_id should be auto-generated, not match custom name")
	}
}

func Test_Create_JSON_Flag_With_From_Branch(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	createBranch(t, cli.Dir, testBranchDevelop)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--json", "--from-branch", testBranchDevelop, "--name", "json-from")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	var result map[string]any

	err := json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}

	if result["from"] != testBranchDevelop {
		t.Errorf("expected from '%s', got %v", testBranchDevelop, result["from"])
	}
}

func Test_Create_Help_Shows_JSON_Flag(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)

	stdout, _, code := cli.Run("create", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}

	AssertContains(t, stdout, "--json")
	AssertContains(t, stdout, "Output as JSON")
}

// Helper function to create a branch in a git repo.
func createBranch(t *testing.T, repoDir, branchName string) {
	t.Helper()

	cmd := testGitCmd("branch", branchName)
	cmd.Dir = repoDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create branch %s: %v\n%s", branchName, err, out)
	}
}

// Helper function to list branches in a git repo.
func listBranches(t *testing.T, repoDir string) []string {
	t.Helper()

	cmd := testGitCmd("branch", "--list", "--format=%(refname:short)")
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}

	var branches []string

	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}

	return branches
}

func Test_Create_Switch_Flag_Outputs_Only_Path(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--switch", "--name", "switch-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Output should be just the path, nothing else
	path := strings.TrimSpace(stdout)

	// Should not contain any other output
	if strings.Contains(path, "Created") {
		t.Errorf("--switch output should not contain 'Created', got: %s", stdout)
	}

	if strings.Contains(path, "name:") {
		t.Errorf("--switch output should not contain 'name:', got: %s", stdout)
	}

	// Should be a valid path containing the worktree name
	if !strings.Contains(path, "switch-test") {
		t.Errorf("expected path to contain 'switch-test', got: %s", path)
	}

	// Path should exist
	_, err := os.Stat(path)
	if err != nil {
		t.Errorf("path should exist: %v", err)
	}
}

func Test_Create_Switch_Short_Flag_Outputs_Only_Path(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "-s", "--name", "short-switch")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	path := strings.TrimSpace(stdout)

	if !strings.Contains(path, "short-switch") {
		t.Errorf("expected path to contain 'short-switch', got: %s", path)
	}

	// Should be just one line
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Errorf("expected single line output, got %d lines: %s", len(lines), stdout)
	}
}

func Test_Create_Switch_Flag_Works_With_Other_Flags(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	// Create a feature branch
	createBranch(t, cli.Dir, "feature")

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := cli.Run("--config", "config.json", "create", "--switch", "--name", "from-feature", "--from-branch", "feature")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	path := strings.TrimSpace(stdout)

	if !strings.Contains(path, "from-feature") {
		t.Errorf("expected path to contain 'from-feature', got: %s", path)
	}
}

func Test_Create_Switch_And_JSON_Flags_Are_Mutually_Exclusive(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)
	initRealGitRepo(t, cli.Dir)

	cli.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := cli.Run("--config", "config.json", "create", "--switch", "--json")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "cannot use --switch and --json together")
}

func Test_Create_Help_Shows_Switch_Flag(t *testing.T) {
	t.Parallel()

	cli := NewCLITester(t)

	stdout, _, code := cli.Run("create", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "--switch")
	AssertContains(t, stdout, "-s")
}
