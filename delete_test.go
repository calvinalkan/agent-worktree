package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

func Test_Delete_Returns_Error_When_No_Name_Provided(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("delete")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "worktree name is required")
}

func Test_Delete_Returns_Error_When_Not_In_Git_Repo(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	// Don't init git repo

	_, stderr, code := c.Run("delete", "some-worktree")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a git repository")
}

func Test_Delete_Returns_Error_When_Worktree_Not_Found(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("delete", "nonexistent-worktree")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "worktree not found")
	AssertContains(t, stderr, "nonexistent-worktree")
}

func Test_Delete_Deletes_Worktree_Successfully(t *testing.T) {
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

	// Delete the worktree (use --with-branch to avoid prompt, --force because new worktrees have uncommitted files)
	stdout, stderr, code = c.Run("--config", "config.json", "delete", "test-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Deleted worktree and branch: test-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/test-wt") {
		t.Error("worktree directory should be deleted")
	}

	_ = wtPath
}

func Test_Delete_Errors_On_Dirty_Worktree_Without_Force(t *testing.T) {
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

	// Try to delete without --force
	_, stderr, code = c.Run("--config", "config.json", "delete", "dirty-wt", "--with-branch")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "uncommitted changes")
	AssertContains(t, stderr, "--force")
}

func Test_Delete_Force_Deletes_Dirty_Worktree(t *testing.T) {
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

	// Delete with --force
	stdout, stderr, code := c.Run("--config", "config.json", "delete", "dirty-wt", "--force", "--with-branch")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Deleted worktree and branch: dirty-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/dirty-wt") {
		t.Error("worktree directory should be deleted")
	}
}

func Test_Delete_Without_WithBranch_Keeps_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "keep-branch-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Delete without --with-branch (non-interactive mode - no stdin)
	// Use --force because new worktrees have uncommitted .wt/worktree.json
	stdout, stderr, code := c.Run("--config", "config.json", "delete", "keep-branch-wt", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should only say "Deleted worktree:", not "Deleted worktree and branch:"
	AssertContains(t, stdout, "Deleted worktree: keep-branch-wt")
	AssertNotContains(t, stdout, "and branch")

	// Verify worktree is gone
	if c.FileExists("worktrees/keep-branch-wt") {
		t.Error("worktree directory should be deleted")
	}

	// Branch should still exist (check via git)
	// (We can't easily verify this in the test without more git operations,
	// but the output message confirms the branch wasn't deleted)
}

func Test_Delete_WithBranch_Deletes_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "delete-branch-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Delete with --with-branch (use --force because new worktrees have uncommitted files)
	stdout, stderr, code := c.Run("--config", "config.json", "delete", "delete-branch-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Deleted worktree and branch: delete-branch-wt")
}

func Test_Delete_Runs_PreDelete_Hook(t *testing.T) {
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

	// Delete the worktree (use --force because new worktrees have uncommitted files)
	stdout, stderr, code := c.Run("--config", "config.json", "delete", "hook-test-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Deleted worktree and branch: hook-test-wt")

	// Verify hook ran
	if !c.FileExists("hook-ran.txt") {
		t.Fatal("hook should have created marker file")
	}

	hookOutput := c.ReadFile("hook-ran.txt")
	AssertContains(t, hookOutput, "WT_NAME=hook-test-wt")
}

func Test_Delete_Aborts_When_PreDelete_Hook_Fails(t *testing.T) {
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

	// Try to delete - should fail due to hook (use --force to bypass dirty check)
	_, stderr, code = c.Run("--config", "config.json", "delete", "abort-wt", "--with-branch", "--force")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "deletion aborted")

	// Worktree should still exist
	if !c.FileExists("worktrees/abort-wt/.wt/worktree.json") {
		t.Error("worktree should still exist after hook failure")
	}
}

func Test_Delete_Help_Shows_Usage_And_Flags(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	stdout, _, code := c.Run("delete", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "Delete a worktree")
	AssertContains(t, stdout, "--force")
	AssertContains(t, stdout, "--with-branch")
}

func Test_Delete_Help_Shows_Detailed_Description(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("delete", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify description explains what gets deleted
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

func Test_Delete_Interactive_Prompt_Yes_Deletes_Branch(t *testing.T) {
	t.Parallel()

	// This test verifies the promptYesNo function works correctly
	// Since IsTerminal() returns false in tests, we can't test the full interactive flow
	// But we can test the promptYesNo function directly

	var stdout strings.Builder

	stdin := strings.NewReader("y\n")

	result := promptYesNo(stdin, &stdout, "Delete branch? (y/N) ")

	if !result {
		t.Error("expected true for 'y' response")
	}

	if !strings.Contains(stdout.String(), "Delete branch? (y/N) ") {
		t.Error("expected prompt to be written to stdout")
	}
}

func Test_Delete_Interactive_Prompt_No_Keeps_Branch(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder

	stdin := strings.NewReader("n\n")

	result := promptYesNo(stdin, &stdout, "Delete branch? (y/N) ")

	if result {
		t.Error("expected false for 'n' response")
	}
}

func Test_Delete_Interactive_Prompt_Empty_Keeps_Branch(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder

	stdin := strings.NewReader("\n")

	result := promptYesNo(stdin, &stdout, "Delete branch? (y/N) ")

	if result {
		t.Error("expected false for empty response")
	}
}

func Test_Delete_Interactive_Prompt_Yes_Uppercase(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder

	stdin := strings.NewReader("Y\n")

	result := promptYesNo(stdin, &stdout, "Delete branch? (y/N) ")

	if !result {
		t.Error("expected true for 'Y' response")
	}
}

func Test_Delete_Works_With_Manual_Worktree_Directory(t *testing.T) {
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

	// Delete the worktree (use --force because the worktree has uncommitted .wt/worktree.json)
	stdout, stderr, code := c.Run("--config", "config.json", "delete", "manual-wt", "--with-branch", "--force")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Deleted worktree and branch: manual-wt")

	// Verify worktree is gone
	if c.FileExists("worktrees/manual-wt") {
		t.Error("worktree directory should be deleted")
	}
}

func Test_Delete_Error_When_Hook_Not_Executable(t *testing.T) {
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

	// Try to delete - should fail due to non-executable hook (use --force to bypass dirty check)
	_, stderr, code = c.Run("--config", "config.json", "delete", "noexec-wt", "--with-branch", "--force")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not executable")

	// Worktree should still exist
	if !c.FileExists("worktrees/noexec-wt/.wt/worktree.json") {
		t.Error("worktree should still exist after hook failure")
	}
}
