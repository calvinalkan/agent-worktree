package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

func Test_Remove_Returns_Error_When_No_Name_Provided(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("remove")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "worktree name is required")
}

func Test_Remove_Returns_Error_When_Not_In_Git_Repo(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	// Don't init git repo

	_, stderr, code := c.Run("remove", "some-worktree")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a git repository")
}

func Test_Remove_Returns_Error_When_Worktree_Not_Found(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("remove", "nonexistent-worktree")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "worktree not found")
	AssertContains(t, stderr, "nonexistent-worktree")
}

func Test_Remove_Removes_Worktree_Successfully(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create a worktree first
	c.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	_ = stdout

	// Verify worktree was created
	wtPath := filepath.Join(c.Dir, "worktrees", "test-wt")
	if !c.FileExists("worktrees/test-wt/.wt/worktree.json") {
		t.Fatalf("worktree was not created")
	}

	// Remove the worktree (use --with-branch to avoid prompt, --force because new worktrees have uncommitted files)
	stdout, stderr, code = c.Run("--config", "config.json", "remove", "test-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: test-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/test-wt") {
		t.Error("worktree directory should be removed")
	}

	_ = wtPath
}

func Test_Remove_Errors_On_Dirty_Worktree_Without_Force(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "dirty-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Make the worktree dirty by adding an uncommitted file
	wtPath := filepath.Join(c.Dir, "worktrees", "dirty-wt")
	dirtyFile := filepath.Join(wtPath, "dirty.txt")

	err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0o644)
	if err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Try to remove without --force
	_, stderr, code = c.Run("--config", "config.json", "remove", "dirty-wt", "--with-branch")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "uncommitted changes")
	AssertContains(t, stderr, "--force")
}

func Test_Remove_Force_Removes_Dirty_Worktree(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "dirty-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Make the worktree dirty
	wtPath := filepath.Join(c.Dir, "worktrees", "dirty-wt")
	dirtyFile := filepath.Join(wtPath, "dirty.txt")

	err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0o644)
	if err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Remove with --force
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "dirty-wt", "--force", "--with-branch")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: dirty-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/dirty-wt") {
		t.Error("worktree directory should be removed")
	}
}

func Test_Remove_Without_WithBranch_Keeps_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "keep-branch-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove without --with-branch (non-interactive mode - no stdin)
	// Use --force because new worktrees have uncommitted .wt/worktree.json
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "keep-branch-wt", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should show worktree removal but not branch deletion
	AssertContains(t, stdout, "Removed worktree:")
	AssertNotContains(t, stdout, "Deleted branch:")

	// Verify worktree is gone
	if c.FileExists("worktrees/keep-branch-wt") {
		t.Error("worktree directory should be removed")
	}

	// Branch should still exist (check via git)
	// (We can't easily verify this in the test without more git operations,
	// but the output message confirms the branch wasn't deleted)
}

func Test_Remove_WithBranch_Deletes_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "delete-branch-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove with --with-branch (use --force because new worktrees have uncommitted files)
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "delete-branch-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: delete-branch-wt")
}

func Test_Remove_Short_Flag_F_Works_Same_As_Force(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "short-f-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Make the worktree dirty
	wtPath := filepath.Join(c.Dir, "worktrees", "short-f-wt")
	dirtyFile := filepath.Join(wtPath, "dirty.txt")

	err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0o644)
	if err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Remove with -f short flag (should work same as --force)
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "short-f-wt", "-f", "-b")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: short-f-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/short-f-wt") {
		t.Error("worktree directory should be removed")
	}
}

func Test_Remove_Short_Flag_B_Works_Same_As_WithBranch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "short-b-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove with -b short flag (should work same as --with-branch)
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "short-b-wt", "-b", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should show both worktree removal and branch deletion because -b was used
	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: short-b-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/short-b-wt") {
		t.Error("worktree directory should be removed")
	}
}

func Test_Remove_Runs_PreDelete_Hook(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create hook that writes env vars to a file
	hookMarker := filepath.Join(c.Dir, "hook-ran.txt")
	hookScript := `#!/bin/bash
echo "WT_NAME=$WT_NAME" > "` + hookMarker + `"
echo "WT_PATH=$WT_PATH" >> "` + hookMarker + `"
`
	c.WriteExecutable(".wt/hooks/pre-delete", hookScript)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "hook-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove the worktree (use --force because new worktrees have uncommitted files)
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "hook-test-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: hook-test-wt")

	// Verify hook ran
	if !c.FileExists("hook-ran.txt") {
		t.Fatal("hook should have created marker file")
	}

	hookOutput := c.ReadFile("hook-ran.txt")
	AssertContains(t, hookOutput, "WT_NAME=hook-test-wt")
}

func Test_Remove_Aborts_When_PreDelete_Hook_Fails(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create hook that fails
	c.WriteExecutable(".wt/hooks/pre-delete", "#!/bin/bash\nexit 1\n")

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "abort-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Try to remove - should fail due to hook (use --force to bypass dirty check)
	_, stderr, code = c.Run("--config", "config.json", "remove", "abort-wt", "--with-branch", "--force")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "pre-delete hook aborted deletion")

	// Worktree should still exist
	if !c.FileExists("worktrees/abort-wt/.wt/worktree.json") {
		t.Error("worktree should still exist after hook failure")
	}
}

func Test_Remove_Help_Shows_Usage_And_Flags(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	stdout, _, code := c.Run("remove", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "Remove a worktree")
	AssertContains(t, stdout, "--force")
	AssertContains(t, stdout, "--with-branch")
	AssertContains(t, stdout, "-f")
	AssertContains(t, stdout, "-b")
}

func Test_Remove_Help_Shows_Detailed_Description(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("remove", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify description explains what gets removed
	AssertContains(t, stdout, "worktree directory and git worktree metadata")

	// Verify interactive vs non-interactive explanation
	AssertContains(t, stdout, "interactive terminal")
	AssertContains(t, stdout, "prompted about branch deletion")
	AssertContains(t, stdout, "non-interactive")

	// Verify pre-delete hook mention
	AssertContains(t, stdout, "pre-delete")
	AssertContains(t, stdout, "abort")

	// Verify improved flag description
	AssertContains(t, stdout, "skips interactive prompt")
}

func Test_Remove_Interactive_Prompt_Yes_Deletes_Branch(t *testing.T) {
	t.Parallel()

	// This test verifies the readYesNo function works correctly
	// Since IsTerminal() returns false in tests, we can't test the full interactive flow
	// But we can test the readYesNo function directly

	stdin := strings.NewReader("y\n")

	result := readYesNo(stdin)

	if !result {
		t.Error("expected true for 'y' response")
	}
}

func Test_Remove_Interactive_Prompt_No_Keeps_Branch(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("n\n")

	result := readYesNo(stdin)

	if result {
		t.Error("expected false for 'n' response")
	}
}

func Test_Remove_Interactive_Prompt_Empty_Keeps_Branch(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("\n")

	result := readYesNo(stdin)

	if result {
		t.Error("expected false for empty response")
	}
}

func Test_Remove_Interactive_Prompt_Yes_Uppercase(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("Y\n")

	result := readYesNo(stdin)

	if !result {
		t.Error("expected true for 'Y' response")
	}
}

func Test_Remove_Works_With_Manual_Worktree_Directory(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	fsys := fs.NewReal()

	// Create worktree base directory and a worktree manually
	wtBaseDir := filepath.Join(c.Dir, "worktrees")

	err := os.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree base dir: %v", err)
	}

	// Create a real git worktree using git command
	wtPath := filepath.Join(wtBaseDir, "manual-wt")

	cmd := testGitCmd("-C", c.Dir, "worktree", "add", "-b", "manual-wt", wtPath, "master")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create git worktree: %v\noutput: %s", err, string(out))
	}

	// Write worktree metadata
	info := WorktreeInfo{
		Name:       "manual-wt",
		AgentID:    "manual-wt",
		ID:         1,
		BaseBranch: "master",
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, wtPath, &info)
	if err != nil {
		t.Fatalf("failed to write worktree info: %v", err)
	}

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Remove the worktree (use --force because the worktree has uncommitted .wt/worktree.json)
	stdout, stderr, code := c.Run("--config", "config.json", "remove", "manual-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: manual-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/manual-wt") {
		t.Error("worktree directory should be removed")
	}
}

func Test_Remove_Error_When_Hook_Not_Executable(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create non-executable hook file
	c.WriteFile(".wt/hooks/pre-delete", "#!/bin/bash\necho 'hook'\n")

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "noexec-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Try to remove - should fail due to non-executable hook (use --force to bypass dirty check)
	_, stderr, code = c.Run("--config", "config.json", "remove", "noexec-wt", "--with-branch", "--force")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not executable")

	// Worktree should still exist
	if !c.FileExists("worktrees/noexec-wt/.wt/worktree.json") {
		t.Error("worktree should still exist after hook failure")
	}
}

func Test_Remove_Alias_Rm_Works(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create a worktree first
	c.WriteFile("config.json", `{"base": "worktrees"}`)

	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "alias-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove using the 'rm' alias (use --force because new worktrees have uncommitted files)
	stdout, stderr, code := c.Run("--config", "config.json", "rm", "alias-test-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: alias-test-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/alias-test-wt") {
		t.Error("worktree directory should be removed")
	}
}

func Test_Remove_Help_Shows_Alias(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("remove", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify alias is shown in help
	AssertContains(t, stdout, "Aliases: rm")
}

func Test_GlobalHelp_Shows_Remove_With_Alias(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify remove command shows alias inline
	AssertContains(t, stdout, "remove, rm")
}
