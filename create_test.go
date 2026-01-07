package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	AssertContains(t, stderr, "git worktree add failed")
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

	_, err := git.CurrentBranch(wtPath)
	if err != nil {
		t.Errorf("worktree is not a valid git repository: %v", err)
	}

	// Verify the branch name matches
	branch, err := git.CurrentBranch(wtPath)
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
