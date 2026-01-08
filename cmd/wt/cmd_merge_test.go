package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// initRepoWithConfig initializes a git repo and commits the config.json file
// so the repo stays clean for merge tests.
func initRepoWithConfig(t *testing.T, c *CLI) {
	t.Helper()

	initRealGitRepo(t, c.Dir)
	c.WriteFile("config.json", `{"base": "worktrees"}`)
	gitCommitFile(t, c.Dir, "config.json")
}

// gitCommitFile commits a file that already exists in the directory.
func gitCommitFile(t *testing.T, dir, filename string) {
	t.Helper()

	cmd := testGitCmd("-C", dir, "add", filename)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd = testGitCmd("-C", dir, "commit", "-m", "Add "+filename)

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

func Test_Merge_Returns_Error_When_Not_In_Worktree(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("merge")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a wt-managed worktree")
}

func Test_Merge_Returns_Error_When_Target_Branch_Not_Exist(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Try to merge into non-existent branch
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge", "--into", "nonexistent-branch")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "validating branches")
	AssertContains(t, stderr, "does not exist")
}

func Test_Merge_Returns_Error_When_Already_On_Target_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Try to merge into itself
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge", "--into", "feature-branch")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "validating branches")
	AssertContains(t, stderr, "already on target branch")
}

func Test_Merge_Returns_Error_When_Worktree_Has_Uncommitted_Changes(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make the worktree dirty
	dirtyFile := filepath.Join(wtPath, "dirty.txt")

	err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0o644)
	if err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Try to merge
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "uncommitted changes")
	AssertContains(t, stderr, "commit or stash")
}

func Test_Merge_Simple_Merge_Success(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Merge
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-branch into master")
	AssertContains(t, stdout, "Removed worktree:")
	AssertContains(t, stdout, "Deleted branch: feature-branch")

	// Verify worktree is removed
	if c.FileExists("worktrees/feature-branch") {
		t.Error("worktree should be removed after merge")
	}

	// Verify commit is on master
	if !gitBranchContainsFile(t, c.Dir, "master", "feature.txt") {
		t.Error("feature.txt should be on master after merge")
	}
}

func Test_Merge_With_Rebase(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit on master (target branch moved ahead)
	gitCommitInDir(t, c.Dir, "master-change.txt", "master content", "Master change")

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Merge
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-branch into master")

	// Verify both commits are on master
	if !gitBranchContainsFile(t, c.Dir, "master", "feature.txt") {
		t.Error("feature.txt should be on master after merge")
	}

	if !gitBranchContainsFile(t, c.Dir, "master", "master-change.txt") {
		t.Error("master-change.txt should still be on master after merge")
	}
}

func Test_Merge_Into_Different_Branch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a develop branch
	cmd := testGitCmd("-C", c.Dir, "branch", "develop")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch develop failed: %v\n%s", err, out)
	}

	// Create a worktree from master
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Merge into develop instead of base_branch (master)
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge", "--into", "develop")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-branch into develop")

	// Verify commit is on develop
	if !gitBranchContainsFile(t, c.Dir, "develop", "feature.txt") {
		t.Error("feature.txt should be on develop after merge")
	}

	// Verify commit is NOT on master
	if gitBranchContainsFile(t, c.Dir, "master", "feature.txt") {
		t.Error("feature.txt should NOT be on master")
	}
}

func Test_Merge_Keep_Flag_Preserves_Worktree(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Merge with --keep
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge", "--keep")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-branch into master")
	AssertContains(t, stdout, "Worktree kept:")
	AssertNotContains(t, stdout, "Removed worktree")

	// Verify worktree still exists
	if !c.FileExists("worktrees/feature-branch/.wt/worktree.json") {
		t.Error("worktree should still exist with --keep flag")
	}
}

func Test_Merge_DryRun_Shows_Plan_Without_Executing(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Dry run merge
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge", "--dry-run")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Dry run")
	AssertContains(t, stdout, "feature-branch")
	AssertContains(t, stdout, "master")
	AssertContains(t, stdout, "Rebase")
	AssertContains(t, stdout, "Fast-forward")
	AssertContains(t, stdout, "Remove worktree")
	AssertContains(t, stdout, "Delete branch")
	AssertContains(t, stdout, "No changes made")

	// Verify nothing actually changed
	if !c.FileExists("worktrees/feature-branch/.wt/worktree.json") {
		t.Error("worktree should still exist after dry-run")
	}

	// Verify commit is NOT on master yet
	if gitBranchContainsFile(t, c.Dir, "master", "feature.txt") {
		t.Error("feature.txt should NOT be on master after dry-run")
	}
}

func Test_Merge_Conflict_Aborts_Cleanly(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Create conflicting changes on both branches
	// First, make a change on master
	gitCommitInDir(t, c.Dir, "conflict.txt", "master version", "Master change")

	// Then make a conflicting change in worktree
	gitCommitInDir(t, wtPath, "conflict.txt", "feature version", "Feature change")

	// Try to merge - should fail with conflict
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 1 {
		t.Errorf("expected exit code 1 for conflict, got %d", code)
	}

	AssertContains(t, stderr, "conflict")

	// Worktree should still exist (not cleaned up)
	if !c.FileExists("worktrees/feature-branch/.wt/worktree.json") {
		t.Error("worktree should still exist after conflict")
	}

	// Verify worktree is clean (rebase was aborted)
	dirty, err := newTestGit().IsDirty(t.Context(), wtPath)
	if err != nil {
		t.Fatalf("failed to check dirty status: %v", err)
	}

	if dirty {
		t.Error("worktree should be clean after rebase abort")
	}
}

func Test_Merge_Runs_PreDelete_Hook(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create hook that writes a marker file
	hookMarker := filepath.Join(c.Dir, "hook-ran.txt")
	hookScript := `#!/bin/bash
echo "WT_NAME=$WT_NAME" > "` + hookMarker + `"
`
	c.WriteExecutable(".wt/hooks/pre-delete", hookScript)

	// Commit the hook file so main repo stays clean
	gitCommitFile(t, c.Dir, ".wt/hooks/pre-delete")

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "hook-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Merge
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify hook ran
	if !c.FileExists("hook-ran.txt") {
		t.Fatal("pre-delete hook should have run")
	}

	hookOutput := c.ReadFile("hook-ran.txt")
	AssertContains(t, hookOutput, "WT_NAME=hook-test")
}

func Test_Merge_Help_Shows_Usage(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("merge", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "Merge")
	AssertContains(t, stdout, "--into")
	AssertContains(t, stdout, "--keep")
	AssertContains(t, stdout, "--dry-run")
}

func Test_Merge_Dirty_Target_Worktree_Errors(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree first (so master worktree becomes tracked)
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make the main repo dirty by MODIFYING a tracked file (not just adding untracked)
	// Modify the already committed config.json file
	err := os.WriteFile(filepath.Join(c.Dir, "config.json"), []byte(`{"base": "modified"}`), 0o644)
	if err != nil {
		t.Fatalf("failed to modify config.json: %v", err)
	}

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Try to merge - should fail because master has uncommitted changes to tracked files
	c2 := NewCLITesterAt(t, wtPath)

	_, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "checking target branch")
	AssertContains(t, stderr, "has uncommitted changes")
}

func Test_Merge_NoCommits_AlreadyUpToDate(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree (no commits added)
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Create a minimal commit to make the worktree clean (wt create adds worktree.json to exclude)
	gitCommitInDir(t, wtPath, "placeholder.txt", "placeholder", "Placeholder commit")

	// Merge
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-branch into master")
}

func Test_Merge_Concurrent_Multiple_Worktrees(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	n := 5
	wtPaths := make([]string, n)

	// Create N worktrees, each with a commit
	for i := range n {
		name := "feature-" + string(rune('a'+i))

		stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", name)
		if code != 0 {
			t.Fatalf("create %s failed: %s", name, stderr)
		}

		wtPaths[i] = extractPath(stdout)

		// Make a unique commit in each worktree
		filename := "file-" + string(rune('a'+i)) + ".txt"
		gitCommitInDir(t, wtPaths[i], filename, "content "+name, "Commit from "+name)
	}

	// Merge all concurrently
	var wg sync.WaitGroup

	errs := make([]error, n)

	for i := range n {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			wt := NewCLITesterAt(t, wtPaths[idx])
			_, stderr, code := wt.Run("--config", "../config.json", "merge")

			if code != 0 {
				errs[idx] = &testError{msg: "feature-" + string(rune('a'+idx)) + ": " + stderr}
			}
		}(i)
	}

	wg.Wait()

	// All should succeed (retry logic should handle contention)
	var failCount int

	for i, err := range errs {
		if err != nil {
			t.Errorf("worktree %d failed: %v", i, err)

			failCount++
		}
	}

	if failCount > 0 {
		t.Errorf("%d/%d merges failed", failCount, n)
	}

	// All commits should be on master
	for i := range n {
		filename := "file-" + string(rune('a'+i)) + ".txt"
		if !gitBranchContainsFile(t, c.Dir, "master", filename) {
			t.Errorf("master should contain %s", filename)
		}
	}

	// All worktrees should be removed
	for i := range n {
		name := "feature-" + string(rune('a'+i))
		if c.FileExists("worktrees/" + name) {
			t.Errorf("worktree %s should be removed", name)
		}
	}
}

// testError is a simple error type for concurrent test errors.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// gitCommitInDir creates a file and commits it in the specified directory.
func gitCommitInDir(t *testing.T, dir, filename, content, message string) {
	t.Helper()

	filePath := filepath.Join(dir, filename)

	// Create parent directories if needed
	if parentDir := filepath.Dir(filePath); parentDir != dir {
		err := os.MkdirAll(parentDir, 0o750)
		if err != nil {
			t.Fatalf("failed to create parent dir: %v", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd := testGitCmd("-C", dir, "add", filename)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd = testGitCmd("-C", dir, "commit", "-m", message)

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

// gitBranchContainsFile checks if a file exists on a branch.
func gitBranchContainsFile(t *testing.T, repoDir, branch, filename string) bool {
	t.Helper()

	cmd := testGitCmd("-C", repoDir, "cat-file", "-e", branch+":"+filename)

	err := cmd.Run()

	return err == nil
}

func Test_Merge_DryRun_With_Keep_Shows_No_Cleanup_Steps(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "feature.txt", "feature content", "Add feature")

	// Dry run merge with --keep
	c2 := NewCLITesterAt(t, wtPath)

	stdout, stderr, code = c2.Run("--config", "../config.json", "merge", "--dry-run", "--keep")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Dry run")
	AssertContains(t, stdout, "Rebase")
	AssertContains(t, stdout, "Fast-forward")
	AssertNotContains(t, stdout, "Remove worktree")
	AssertNotContains(t, stdout, "Delete branch")
}

func Test_Merge_From_Nested_Worktree_To_Another_Worktree(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create first worktree (feature-a)
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-a")
	if code != 0 {
		t.Fatalf("create feature-a failed: %s", stderr)
	}

	wtPathA := extractPath(stdout)

	// Make a commit on feature-a
	gitCommitInDir(t, wtPathA, "a.txt", "content a", "Commit A")

	// Create second worktree from feature-a branch
	c2 := NewCLITesterAt(t, wtPathA)

	stdout, stderr, code = c2.Run("--config", "../config.json", "create", "--name", "feature-b", "--from-branch", "feature-a")
	if code != 0 {
		t.Fatalf("create feature-b failed: %s", stderr)
	}

	wtPathB := extractPath(stdout)

	// Make a commit on feature-b
	gitCommitInDir(t, wtPathB, "b.txt", "content b", "Commit B")

	// Merge feature-b into feature-a
	c3 := NewCLITesterAt(t, wtPathB)

	stdout, stderr, code = c3.Run("--config", "../../config.json", "merge")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Merged feature-b into feature-a")

	// Verify b.txt is now on feature-a
	if !gitBranchContainsFile(t, c.Dir, "feature-a", "b.txt") {
		t.Error("b.txt should be on feature-a after merge")
	}

	// feature-b worktree should be removed
	if c.FileExists("worktrees/feature-b") {
		t.Error("feature-b worktree should be removed")
	}
}

func Test_Merge_GlobalHelp_Shows_Merge_Command(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "merge")
}

func Test_Merge_DryRun_Shows_Correct_Commit_Count(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make 3 commits in the worktree
	gitCommitInDir(t, wtPath, "file1.txt", "content 1", "Commit 1")
	gitCommitInDir(t, wtPath, "file2.txt", "content 2", "Commit 2")
	gitCommitInDir(t, wtPath, "file3.txt", "content 3", "Commit 3")

	// Dry run
	c2 := NewCLITesterAt(t, wtPath)

	stdout, _, code = c2.Run("--config", "../config.json", "merge", "--dry-run")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Should show 3 commits
	AssertContains(t, stdout, "3 commits")
}

func Test_Merge_DryRun_Shows_Single_Commit_Grammar(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make 1 commit in the worktree
	gitCommitInDir(t, wtPath, "file1.txt", "content 1", "Commit 1")

	// Dry run
	c2 := NewCLITesterAt(t, wtPath)

	stdout, _, code = c2.Run("--config", "../config.json", "merge", "--dry-run")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Should show "1 commit" (singular)
	AssertContains(t, stdout, "1 commit")
	AssertNotContains(t, stdout, "1 commits")
}

func Test_Merge_DryRun_Checks_Pass(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRepoWithConfig(t, c)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "feature-branch")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Make a commit in the worktree
	gitCommitInDir(t, wtPath, "file.txt", "content", "Commit")

	// Dry run
	c2 := NewCLITesterAt(t, wtPath)

	stdout, _, code = c2.Run("--config", "../config.json", "merge", "--dry-run")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Should show checks passed
	AssertContains(t, stdout, "✓ Current worktree is clean")
	AssertContains(t, stdout, "✓ Target branch 'master' exists")
}

func Test_Merge_Includes_Merge_In_Global_Commands(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Check merge command is listed
	lines := strings.Split(stdout, "\n")
	foundMerge := false

	for _, line := range lines {
		if strings.Contains(line, "merge") && strings.Contains(line, "Merge") {
			foundMerge = true

			break
		}
	}

	if !foundMerge {
		t.Error("merge command should be listed in global help")
	}
}
