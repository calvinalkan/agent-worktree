package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

const testAgentIDBraveOwl = "brave-owl"

func Test_List_Returns_Error_When_Not_In_Git_Repo(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	// Don't initialize git repo

	_, stderr, code := c.Run("list")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a git repository")
}

func Test_List_Returns_Empty_When_No_Worktrees(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	stdout, stderr, code := c.Run("list")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Empty output for no worktrees
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got: %q", stdout)
	}
}

func Test_List_Shows_Worktrees_In_Table_Format(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	repoDir := initRealGitRepo(t, c.Dir)
	fsys := fs.NewReal()

	// Create worktree base directory and a worktree
	wtBaseDir := filepath.Join(c.Dir, "worktrees")

	err := os.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree base dir: %v", err)
	}

	wtPath := filepath.Join(wtBaseDir, "swift-fox")

	err = os.MkdirAll(wtPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Write worktree metadata
	info := WorktreeInfo{
		Name:       "swift-fox",
		AgentID:    "swift-fox",
		ID:         1,
		BaseBranch: "main",
		Created:    time.Now().UTC().Add(-2 * time.Hour),
	}

	err = writeWorktreeInfo(fsys, wtPath, &info)
	if err != nil {
		t.Fatalf("failed to write worktree info: %v", err)
	}

	// Use relative base path so it finds our worktrees dir
	c.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := c.Run("--config", "config.json", "list")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Check table header
	AssertContains(t, stdout, "NAME")
	AssertContains(t, stdout, "PATH")
	AssertContains(t, stdout, "CREATED")

	// Check worktree appears
	AssertContains(t, stdout, "swift-fox")

	_ = repoDir
}

func Test_List_Shows_Worktrees_In_JSON_Format(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	fsys := fs.NewReal()

	// Create worktree base directory and a worktree
	wtBaseDir := filepath.Join(c.Dir, "worktrees")

	err := os.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree base dir: %v", err)
	}

	wtPath := filepath.Join(wtBaseDir, testAgentIDBraveOwl)

	err = os.MkdirAll(wtPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	created := time.Date(2025, 1, 5, 10, 30, 0, 0, time.UTC)
	info := WorktreeInfo{
		Name:       testAgentIDBraveOwl,
		AgentID:    testAgentIDBraveOwl,
		ID:         42,
		BaseBranch: "develop",
		Created:    created,
	}

	err = writeWorktreeInfo(fsys, wtPath, &info)
	if err != nil {
		t.Fatalf("failed to write worktree info: %v", err)
	}

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := c.Run("--config", "config.json", "list", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Parse JSON output
	var worktrees []jsonWorktree

	err = json.Unmarshal([]byte(stdout), &worktrees)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, stdout)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}

	wt := worktrees[0]

	if wt.Name != testAgentIDBraveOwl {
		t.Errorf("expected name 'brave-owl', got %q", wt.Name)
	}

	if wt.AgentID != testAgentIDBraveOwl {
		t.Errorf("expected agent_id 'brave-owl', got %q", wt.AgentID)
	}

	if wt.ID != 42 {
		t.Errorf("expected id 42, got %d", wt.ID)
	}

	if wt.BaseBranch != "develop" {
		t.Errorf("expected base_branch 'develop', got %q", wt.BaseBranch)
	}
}

func Test_List_Shows_Multiple_Worktrees(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	fsys := fs.NewReal()

	// Create worktree base directory
	wtBaseDir := filepath.Join(c.Dir, "worktrees")

	err := os.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree base dir: %v", err)
	}

	// Create two worktrees
	worktreesData := []struct {
		name    string
		agentID string
		id      int
	}{
		{"wt-one", "swift-fox", 1},
		{"wt-two", testAgentIDBraveOwl, 2},
	}

	for _, wtData := range worktreesData {
		wtPath := filepath.Join(wtBaseDir, wtData.name)

		err = os.MkdirAll(wtPath, 0o750)
		if err != nil {
			t.Fatalf("failed to create worktree dir: %v", err)
		}

		info := WorktreeInfo{
			Name:       wtData.name,
			AgentID:    wtData.agentID,
			ID:         wtData.id,
			BaseBranch: "main",
			Created:    time.Now().UTC(),
		}

		err = writeWorktreeInfo(fsys, wtPath, &info)
		if err != nil {
			t.Fatalf("failed to write worktree info: %v", err)
		}
	}

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := c.Run("--config", "config.json", "list", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	var worktrees []jsonWorktree

	err = json.Unmarshal([]byte(stdout), &worktrees)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}
}

func Test_List_Skips_Non_Managed_Directories(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	fsys := fs.NewReal()

	// Create worktree base directory
	wtBaseDir := filepath.Join(c.Dir, "worktrees")

	err := os.MkdirAll(wtBaseDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree base dir: %v", err)
	}

	// Create a managed worktree
	managedPath := filepath.Join(wtBaseDir, "managed")

	err = os.MkdirAll(managedPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create managed dir: %v", err)
	}

	info := WorktreeInfo{
		Name:       "managed",
		AgentID:    "swift-fox",
		ID:         1,
		BaseBranch: "main",
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, managedPath, &info)
	if err != nil {
		t.Fatalf("failed to write worktree info: %v", err)
	}

	// Create an unmanaged directory (no .wt/worktree.json)
	unmanagedPath := filepath.Join(wtBaseDir, "unmanaged")

	err = os.MkdirAll(unmanagedPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create unmanaged dir: %v", err)
	}

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	stdout, stderr, code := c.Run("--config", "config.json", "list", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	var worktrees []jsonWorktree

	err = json.Unmarshal([]byte(stdout), &worktrees)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should only have the managed worktree
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 && worktrees[0].Name != "managed" {
		t.Errorf("expected worktree 'managed', got %q", worktrees[0].Name)
	}
}

func Test_List_JSON_Empty_Returns_Empty_Array(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	stdout, stderr, code := c.Run("list", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should be valid JSON empty array
	var worktrees []jsonWorktree

	err := json.Unmarshal([]byte(stdout), &worktrees)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %q", err, stdout)
	}

	if len(worktrees) != 0 {
		t.Errorf("expected empty array, got %d items", len(worktrees))
	}
}

func Test_formatAge_Returns_Correct_Strings(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		created  time.Time
		contains string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-1 * time.Minute), "1 minute ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-1 * time.Hour), "1 hour ago"},
		{now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{now.Add(-1 * 24 * time.Hour), "1 day ago"},
	}

	for _, tt := range tests {
		result := formatAge(tt.created)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("formatAge(%v) = %q, want to contain %q", tt.created, result, tt.contains)
		}
	}
}

func Test_findWorktreesWithPaths_Returns_Paths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create a worktree
	wtPath := filepath.Join(dir, "my-worktree")

	err := os.MkdirAll(wtPath, 0o750)
	if err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	info := WorktreeInfo{
		Name:       "my-worktree",
		AgentID:    "swift-fox",
		ID:         1,
		BaseBranch: "main",
		Created:    time.Now().UTC(),
	}

	err = writeWorktreeInfo(fsys, wtPath, &info)
	if err != nil {
		t.Fatalf("failed to write worktree info: %v", err)
	}

	worktrees, err := findWorktreesWithPaths(fsys, dir)
	if err != nil {
		t.Fatalf("findWorktreesWithPaths failed: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}

	if worktrees[0].Path != wtPath {
		t.Errorf("expected path %q, got %q", wtPath, worktrees[0].Path)
	}

	if worktrees[0].Name != "my-worktree" {
		t.Errorf("expected name 'my-worktree', got %q", worktrees[0].Name)
	}
}
