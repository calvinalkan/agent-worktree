package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

const testBaseBranchMain = "master"

func Test_writeWorktreeInfo_Creates_Wt_Directory_And_Json_File(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	info := WorktreeInfo{
		Name:       "test-worktree",
		AgentID:    "swift-fox",
		ID:         42,
		BaseBranch: testBaseBranchMain,
		Created:    time.Date(2025, 1, 7, 16, 30, 0, 0, time.UTC),
	}

	err := writeWorktreeInfo(fsys, dir, &info)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify .wt directory was created
	wtDir := filepath.Join(dir, ".wt")

	stat, err := os.Stat(wtDir)
	if err != nil {
		t.Fatalf("expected .wt directory to exist, got error: %v", err)
	}

	if !stat.IsDir() {
		t.Error(".wt should be a directory")
	}

	// Verify worktree.json was created
	jsonPath := filepath.Join(wtDir, "worktree.json")

	_, err = os.Stat(jsonPath)
	if err != nil {
		t.Fatalf("expected worktree.json to exist, got error: %v", err)
	}
}

func Test_writeWorktreeInfo_Writes_Valid_Json(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	created := time.Date(2025, 1, 7, 16, 30, 0, 0, time.UTC)
	info := WorktreeInfo{
		Name:       "my-feature",
		AgentID:    "brave-owl",
		ID:         7,
		BaseBranch: "develop",
		Created:    created,
	}

	err := writeWorktreeInfo(fsys, dir, &info)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Read and verify content
	jsonPath := filepath.Join(dir, ".wt", "worktree.json")

	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read worktree.json: %v", err)
	}

	// Check JSON contains expected fields
	jsonStr := string(content)

	if !strings.Contains(jsonStr, `"name": "my-feature"`) {
		t.Error("JSON should contain name field")
	}

	if !strings.Contains(jsonStr, `"agent_id": "brave-owl"`) {
		t.Error("JSON should contain agent_id field")
	}

	if !strings.Contains(jsonStr, `"id": 7`) {
		t.Error("JSON should contain id field")
	}

	if !strings.Contains(jsonStr, `"base_branch": "develop"`) {
		t.Error("JSON should contain base_branch field")
	}

	if !strings.Contains(jsonStr, `"created": "2025-01-07T16:30:00Z"`) {
		t.Error("JSON should contain created field")
	}
}

func Test_readWorktreeInfo_Reads_Valid_Json(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create .wt directory and write JSON manually
	wtDir := filepath.Join(dir, ".wt")

	err := os.MkdirAll(wtDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create .wt directory: %v", err)
	}

	jsonContent := `{
  "name": "test-worktree",
  "agent_id": "quick-cat",
  "id": 99,
  "base_branch": "master",
  "created": "2025-01-07T12:00:00Z"
}`

	jsonPath := filepath.Join(wtDir, "worktree.json")

	err = os.WriteFile(jsonPath, []byte(jsonContent), 0o644)
	if err != nil {
		t.Fatalf("failed to write worktree.json: %v", err)
	}

	info, err := readWorktreeInfo(fsys, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Name != "test-worktree" {
		t.Errorf("expected name 'test-worktree', got %q", info.Name)
	}

	if info.AgentID != "quick-cat" {
		t.Errorf("expected agent_id 'quick-cat', got %q", info.AgentID)
	}

	if info.ID != 99 {
		t.Errorf("expected id 99, got %d", info.ID)
	}

	if info.BaseBranch != testBaseBranchMain {
		t.Errorf("expected base_branch %q, got %q", testBaseBranchMain, info.BaseBranch)
	}

	expectedTime := time.Date(2025, 1, 7, 12, 0, 0, 0, time.UTC)
	if !info.Created.Equal(expectedTime) {
		t.Errorf("expected created %v, got %v", expectedTime, info.Created)
	}
}

func Test_readWorktreeInfo_Returns_Error_When_File_Not_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	_, err := readWorktreeInfo(fsys, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Error should mention the file
	if !strings.Contains(err.Error(), "worktree.json") {
		t.Errorf("error should mention worktree.json: %v", err)
	}
}

func Test_readWorktreeInfo_Returns_Error_When_Invalid_Json(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create .wt directory and write invalid JSON
	wtDir := filepath.Join(dir, ".wt")

	err := os.MkdirAll(wtDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create .wt directory: %v", err)
	}

	jsonPath := filepath.Join(wtDir, "worktree.json")

	err = os.WriteFile(jsonPath, []byte("not valid json"), 0o644)
	if err != nil {
		t.Fatalf("failed to write worktree.json: %v", err)
	}

	_, err = readWorktreeInfo(fsys, dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error should mention parsing: %v", err)
	}
}

func Test_writeWorktreeInfo_And_readWorktreeInfo_Roundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	created := time.Date(2025, 1, 7, 10, 30, 0, 0, time.UTC)
	original := WorktreeInfo{
		Name:       "roundtrip-test",
		AgentID:    "calm-deer",
		ID:         123,
		BaseBranch: "feature",
		Created:    created,
	}

	err := writeWorktreeInfo(fsys, dir, &original)
	if err != nil {
		t.Fatalf("writeWorktreeInfo failed: %v", err)
	}

	read, err := readWorktreeInfo(fsys, dir)
	if err != nil {
		t.Fatalf("readWorktreeInfo failed: %v", err)
	}

	if read.Name != original.Name {
		t.Errorf("name mismatch: expected %q, got %q", original.Name, read.Name)
	}

	if read.AgentID != original.AgentID {
		t.Errorf("agent_id mismatch: expected %q, got %q", original.AgentID, read.AgentID)
	}

	if read.ID != original.ID {
		t.Errorf("id mismatch: expected %d, got %d", original.ID, read.ID)
	}

	if read.BaseBranch != original.BaseBranch {
		t.Errorf("base_branch mismatch: expected %q, got %q", original.BaseBranch, read.BaseBranch)
	}

	if !read.Created.Equal(original.Created) {
		t.Errorf("created mismatch: expected %v, got %v", original.Created, read.Created)
	}
}

func Test_findWorktrees_Returns_Empty_When_Dir_Not_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	searchDir := filepath.Join(dir, "nonexistent")

	worktrees, err := findWorktrees(fsys, searchDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(worktrees) != 0 {
		t.Errorf("expected empty slice, got %d worktrees", len(worktrees))
	}
}

func Test_findWorktrees_Returns_Empty_When_Dir_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	worktrees, err := findWorktrees(fsys, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(worktrees) != 0 {
		t.Errorf("expected empty slice, got %d worktrees", len(worktrees))
	}
}

func Test_findWorktrees_Finds_Managed_Worktrees(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create two worktrees with metadata
	wt1Path := filepath.Join(dir, "wt-1")

	err := os.MkdirAll(wt1Path, 0o750)
	if err != nil {
		t.Fatalf("failed to create wt-1: %v", err)
	}

	wt1Info := WorktreeInfo{
		Name:       "wt-1",
		AgentID:    "swift-fox",
		ID:         1,
		BaseBranch: testBaseBranchMain,
		Created:    time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC),
	}

	err = writeWorktreeInfo(fsys, wt1Path, &wt1Info)
	if err != nil {
		t.Fatalf("failed to write wt-1 info: %v", err)
	}

	wt2Path := filepath.Join(dir, "wt-2")

	err = os.MkdirAll(wt2Path, 0o750)
	if err != nil {
		t.Fatalf("failed to create wt-2: %v", err)
	}

	wt2Info := WorktreeInfo{
		Name:       "wt-2",
		AgentID:    "brave-owl",
		ID:         2,
		BaseBranch: "develop",
		Created:    time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC),
	}

	err = writeWorktreeInfo(fsys, wt2Path, &wt2Info)
	if err != nil {
		t.Fatalf("failed to write wt-2 info: %v", err)
	}

	worktrees, err := findWorktrees(fsys, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Check that we found both
	foundWT1 := false
	foundWT2 := false

	for _, wt := range worktrees {
		if wt.Name == "wt-1" && wt.ID == 1 {
			foundWT1 = true
		}

		if wt.Name == "wt-2" && wt.ID == 2 {
			foundWT2 = true
		}
	}

	if !foundWT1 {
		t.Error("wt-1 not found in results")
	}

	if !foundWT2 {
		t.Error("wt-2 not found in results")
	}
}

func Test_findWorktrees_Skips_Non_Managed_Directories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create one managed worktree
	wtPath := filepath.Join(dir, "managed")

	err := os.MkdirAll(wtPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create managed: %v", err)
	}

	wtInfo := WorktreeInfo{
		Name:       "managed",
		AgentID:    "calm-deer",
		ID:         1,
		BaseBranch: testBaseBranchMain,
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, wtPath, &wtInfo)
	if err != nil {
		t.Fatalf("failed to write managed info: %v", err)
	}

	// Create an unmanaged directory (no .wt/worktree.json)
	unmanagedPath := filepath.Join(dir, "unmanaged")

	err = os.MkdirAll(unmanagedPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create unmanaged: %v", err)
	}

	// Create a regular file (should be skipped)
	regularFile := filepath.Join(dir, "regular-file.txt")

	err = os.WriteFile(regularFile, []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	worktrees, err := findWorktrees(fsys, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 && worktrees[0].Name != "managed" {
		t.Errorf("expected worktree name 'managed', got %q", worktrees[0].Name)
	}
}

func Test_findWorktrees_Handles_Mixed_Content(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create a directory with .wt but invalid JSON
	invalidPath := filepath.Join(dir, "invalid")
	invalidWtDir := filepath.Join(invalidPath, ".wt")

	err := os.MkdirAll(invalidWtDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create invalid/.wt: %v", err)
	}

	err = os.WriteFile(filepath.Join(invalidWtDir, "worktree.json"), []byte("invalid json"), 0o644)
	if err != nil {
		t.Fatalf("failed to write invalid worktree.json: %v", err)
	}

	// Create a valid worktree
	validPath := filepath.Join(dir, "valid")

	err = os.MkdirAll(validPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create valid: %v", err)
	}

	validInfo := WorktreeInfo{
		Name:       "valid",
		AgentID:    "keen-fox",
		ID:         1,
		BaseBranch: testBaseBranchMain,
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, validPath, &validInfo)
	if err != nil {
		t.Fatalf("failed to write valid info: %v", err)
	}

	// findWorktrees should skip the invalid one and return the valid one
	worktrees, err := findWorktrees(fsys, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 && worktrees[0].Name != "valid" {
		t.Errorf("expected worktree name 'valid', got %q", worktrees[0].Name)
	}
}

func Test_writeWorktreeInfo_Returns_Error_When_Cannot_Create_Directory(t *testing.T) {
	t.Parallel()

	// Try to write to a path that's actually a file
	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create a file where we would normally create .wt directory
	wtPath := filepath.Join(dir, "worktree")
	wtDirAsFile := filepath.Join(wtPath, ".wt")

	err := os.MkdirAll(wtPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Create .wt as a file instead of directory
	err = os.WriteFile(wtDirAsFile, []byte("i am a file"), 0o644)
	if err != nil {
		t.Fatalf("failed to create .wt as file: %v", err)
	}

	info := WorktreeInfo{
		Name:       "test",
		AgentID:    "test-id",
		ID:         1,
		BaseBranch: testBaseBranchMain,
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, wtPath, &info)
	if err == nil {
		t.Error("expected error when .wt is a file, got nil")
	}
}

func Test_readWorktreeInfo_Wrapped_Error_Contains_Original(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Directory exists but no worktree.json
	_, err := readWorktreeInfo(fsys, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should be wrapped and contain os.ErrNotExist
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to wrap os.ErrNotExist, got: %v", err)
	}
}
