package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeTestFile writes content to a file for testing purposes.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

// statTestPath checks if a path exists for testing purposes.
func statTestPath(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// initRealGitRepo creates a real git repository in the given directory.
// Returns the repo path.
func initRealGitRepo(t *testing.T, dir string) string {
	t.Helper()

	// Initialize git repo with main as initial branch
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config email failed: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config name failed: %v\n%s", err, out)
	}

	// Disable commit signing for tests
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	cmd.Dir = dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config gpgsign failed: %v\n%s", err, out)
	}

	// Create a file and commit it
	testFile := filepath.Join(dir, "README.md")
	writeTestFile(t, testFile, "# Test\n")

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	return dir
}

func Test_gitRepoRoot_Returns_Root_When_In_Repo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	root, err := gitRepoRoot(repoPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Normalize paths for comparison
	expected, _ := filepath.EvalSymlinks(repoPath)
	actual, _ := filepath.EvalSymlinks(root)

	if actual != expected {
		t.Errorf("expected root %q, got %q", expected, actual)
	}
}

func Test_gitRepoRoot_Returns_Root_When_In_Subdir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Create a subdirectory
	subdir := filepath.Join(repoPath, "subdir", "nested")

	err := os.MkdirAll(subdir, 0o750)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	root, err := gitRepoRoot(subdir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Normalize paths for comparison
	expected, _ := filepath.EvalSymlinks(repoPath)
	actual, _ := filepath.EvalSymlinks(root)

	if actual != expected {
		t.Errorf("expected root %q, got %q", expected, actual)
	}
}

func Test_gitRepoRoot_Returns_Error_When_Not_In_Repo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Don't initialize git repo

	_, err := gitRepoRoot(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotGitRepository) {
		t.Errorf("expected ErrNotGitRepository, got: %v", err)
	}
}

func Test_gitCurrentBranch_Returns_Branch_Name(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initRealGitRepo(t, dir)

	branch, err := gitCurrentBranch(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if branch != "main" {
		t.Errorf("expected branch 'main', got %q", branch)
	}
}

func Test_gitCurrentBranch_Returns_Branch_After_Switch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initRealGitRepo(t, dir)

	// Create and switch to new branch
	cmd := exec.Command("git", "switch", "-c", "feature-test")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git switch -c failed: %v\n%s", err, out)
	}

	branch, err := gitCurrentBranch(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if branch != "feature-test" {
		t.Errorf("expected branch 'feature-test', got %q", branch)
	}
}

func Test_gitIsDirty_Returns_False_When_Clean(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initRealGitRepo(t, dir)

	dirty, err := gitIsDirty(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if dirty {
		t.Error("expected clean repo, got dirty")
	}
}

func Test_gitIsDirty_Returns_True_When_Modified(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initRealGitRepo(t, dir)

	// Modify a file
	testFile := filepath.Join(dir, "README.md")
	writeTestFile(t, testFile, "# Modified\n")

	dirty, err := gitIsDirty(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !dirty {
		t.Error("expected dirty repo, got clean")
	}
}

func Test_gitIsDirty_Returns_True_When_Untracked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initRealGitRepo(t, dir)

	// Add untracked file
	newFile := filepath.Join(dir, "new-file.txt")
	writeTestFile(t, newFile, "new content\n")

	dirty, err := gitIsDirty(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !dirty {
		t.Error("expected dirty repo, got clean")
	}
}

func Test_gitWorktreeAdd_Creates_Worktree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "worktree-test")

	err := gitWorktreeAdd(repoPath, wtPath, "feature-branch", "main")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify worktree exists
	if !statTestPath(wtPath) {
		t.Error("worktree directory was not created")
	}

	// Verify branch was created
	branch, err := gitCurrentBranch(wtPath)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	if branch != "feature-branch" {
		t.Errorf("expected branch 'feature-branch', got %q", branch)
	}
}

func Test_gitWorktreeAdd_Returns_Error_When_Branch_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "worktree-test")

	// First create should succeed
	err := gitWorktreeAdd(repoPath, wtPath, "feature-branch", "main")
	if err != nil {
		t.Fatalf("first worktree add failed: %v", err)
	}

	// Second create with same branch should fail
	wtPath2 := filepath.Join(dir, "worktree-test-2")

	err = gitWorktreeAdd(repoPath, wtPath2, "feature-branch", "main")
	if err == nil {
		t.Error("expected error for duplicate branch, got nil")
	}
}

func Test_gitWorktreeAdd_Creates_Worktree_From_Different_Branch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Create a develop branch
	cmd := exec.Command("git", "branch", "develop")
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch develop failed: %v\n%s", err, out)
	}

	wtPath := filepath.Join(dir, "worktree-test")

	// Create worktree from develop branch
	err = gitWorktreeAdd(repoPath, wtPath, "feature-from-develop", "develop")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify worktree exists
	if !statTestPath(wtPath) {
		t.Error("worktree directory was not created")
	}
}

func Test_gitWorktreeRemove_Removes_Worktree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "worktree-test")

	// Create worktree first
	err := gitWorktreeAdd(repoPath, wtPath, "feature-branch", "main")
	if err != nil {
		t.Fatalf("worktree add failed: %v", err)
	}

	// Remove worktree
	err = gitWorktreeRemove(repoPath, wtPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify worktree was removed
	if statTestPath(wtPath) {
		t.Error("worktree directory still exists")
	}
}

func Test_gitWorktreeRemove_Returns_Error_When_Dirty_Without_Force(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "worktree-test")

	// Create worktree
	err := gitWorktreeAdd(repoPath, wtPath, "feature-branch", "main")
	if err != nil {
		t.Fatalf("worktree add failed: %v", err)
	}

	// Make worktree dirty
	newFile := filepath.Join(wtPath, "dirty-file.txt")
	writeTestFile(t, newFile, "dirty\n")

	// Try to remove without force - should fail
	err = gitWorktreeRemove(repoPath, wtPath, false)
	if err == nil {
		t.Error("expected error for dirty worktree, got nil")
	}
}

func Test_gitWorktreeRemove_Removes_Dirty_Worktree_With_Force(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "worktree-test")

	// Create worktree
	err := gitWorktreeAdd(repoPath, wtPath, "feature-branch", "main")
	if err != nil {
		t.Fatalf("worktree add failed: %v", err)
	}

	// Make worktree dirty
	newFile := filepath.Join(wtPath, "dirty-file.txt")
	writeTestFile(t, newFile, "dirty\n")

	// Remove with force - should succeed
	err = gitWorktreeRemove(repoPath, wtPath, true)
	if err != nil {
		t.Fatalf("expected no error with force, got: %v", err)
	}
}

func Test_gitWorktreePrune_Succeeds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	err := gitWorktreePrune(repoPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func Test_gitBranchDelete_Deletes_Branch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Create a branch
	cmd := exec.Command("git", "branch", "feature-to-delete")
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch failed: %v\n%s", err, out)
	}

	// Delete the branch
	err = gitBranchDelete(repoPath, "feature-to-delete", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify branch was deleted
	cmd = exec.Command("git", "branch", "--list", "feature-to-delete")
	cmd.Dir = repoPath
	out, _ = cmd.Output()

	if len(out) > 0 {
		t.Error("branch still exists after deletion")
	}
}

func Test_gitBranchDelete_Returns_Error_When_Branch_Not_Merged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Create and switch to a branch
	cmd := exec.Command("git", "switch", "-c", "unmerged-branch")
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git switch -c failed: %v\n%s", err, out)
	}

	// Make a commit on this branch
	testFile := filepath.Join(repoPath, "new-file.txt")
	writeTestFile(t, testFile, "new content\n")

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "commit", "-m", "new commit")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Go back to main
	cmd = exec.Command("git", "switch", "main")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git switch main failed: %v\n%s", err, out)
	}

	// Try to delete unmerged branch without force - should fail
	err = gitBranchDelete(repoPath, "unmerged-branch", false)
	if err == nil {
		t.Error("expected error for unmerged branch, got nil")
	}
}

func Test_gitBranchDelete_Force_Deletes_Unmerged_Branch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Create and switch to a branch
	cmd := exec.Command("git", "switch", "-c", "unmerged-branch")
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git switch -c failed: %v\n%s", err, out)
	}

	// Make a commit on this branch
	testFile := filepath.Join(repoPath, "new-file.txt")
	writeTestFile(t, testFile, "new content\n")

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "commit", "-m", "new commit")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Go back to main
	cmd = exec.Command("git", "switch", "main")
	cmd.Dir = repoPath

	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git switch main failed: %v\n%s", err, out)
	}

	// Force delete unmerged branch - should succeed
	err = gitBranchDelete(repoPath, "unmerged-branch", true)
	if err != nil {
		t.Fatalf("expected no error with force, got: %v", err)
	}
}

func Test_gitWorktreeList_Returns_Worktree_Paths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)

	// Initially should have just the main worktree
	paths, err := gitWorktreeList(repoPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(paths) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(paths))
	}

	// Add a worktree
	wtPath := filepath.Join(dir, "worktree-1")

	err = gitWorktreeAdd(repoPath, wtPath, "branch-1", "main")
	if err != nil {
		t.Fatalf("worktree add failed: %v", err)
	}

	// Now should have 2 worktrees
	paths, err = gitWorktreeList(repoPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(paths))
	}
}

func Test_gitWorktreeList_Includes_Worktree_Path(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repoPath := initRealGitRepo(t, dir)
	wtPath := filepath.Join(dir, "my-worktree")

	err := gitWorktreeAdd(repoPath, wtPath, "my-branch", "main")
	if err != nil {
		t.Fatalf("worktree add failed: %v", err)
	}

	paths, err := gitWorktreeList(repoPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Normalize paths for comparison
	wtPathNorm, _ := filepath.EvalSymlinks(wtPath)
	found := false

	for _, p := range paths {
		pNorm, _ := filepath.EvalSymlinks(p)
		if pNorm == wtPathNorm {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("worktree path %q not found in list: %v", wtPath, paths)
	}
}
