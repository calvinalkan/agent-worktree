package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func Test_Info_Shows_Help_When_Help_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("info", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "Usage: wt info")
	AssertContains(t, stdout, "--json")
	AssertContains(t, stdout, "--field")
}

func Test_Info_Returns_Error_When_Not_In_Git_Repo(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	// Don't init git repo

	_, stderr, code := c.Run("info")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not a git repository")
}

func Test_Info_Returns_Error_When_Not_In_Worktree(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	_, stderr, code := c.Run("info")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "not in a wt-managed worktree")
}

func Test_Info_Shows_Worktree_Info_In_Text_Format(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "info-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Run info from inside the worktree
	wtPath := filepath.Join(c.Dir, "worktrees", "info-test-wt")

	stdout, stderr, code := c.RunWithInput(nil, "--config", "../config.json", "-C", wtPath, "info")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        info-test-wt")
	AssertContains(t, stdout, "agent_id:")
	AssertContains(t, stdout, "id:")
	AssertContains(t, stdout, "path:")
	AssertContains(t, stdout, "base_branch: master")
	AssertContains(t, stdout, "created:")
}

func Test_Info_Shows_Worktree_Info_In_JSON_Format(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "json-info-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Run info from inside the worktree
	wtPath := filepath.Join(c.Dir, "worktrees", "json-info-wt")

	stdout, stderr, code := c.RunWithInput(nil, "--config", "../config.json", "-C", wtPath, "info", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Parse JSON output
	var info infoJSON

	err := json.Unmarshal([]byte(stdout), &info)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, stdout)
	}

	if info.Name != "json-info-wt" {
		t.Errorf("expected name 'json-info-wt', got %q", info.Name)
	}

	if info.BaseBranch != "master" {
		t.Errorf("expected base_branch 'master', got %q", info.BaseBranch)
	}
}

func Test_Info_Field_Returns_Single_Value(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "field-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := filepath.Join(c.Dir, "worktrees", "field-test-wt")

	// Test each field
	testCases := []struct {
		field    string
		contains string
	}{
		{"name", "field-test-wt"},
		{"base_branch", "master"},
		{"id", ""}, // Can't predict ID value
	}

	for _, tc := range testCases {
		stdout, stderr, code := c.RunWithInput(nil, "--config", "../config.json", "-C", wtPath, "info", "--field", tc.field)

		if code != 0 {
			t.Errorf("field %s: expected exit code 0, got %d\nstderr: %s", tc.field, code, stderr)

			continue
		}

		if tc.contains != "" {
			AssertContains(t, stdout, tc.contains)
		}
	}
}

func Test_Info_Field_Returns_Error_For_Invalid_Field(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "invalid-field-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := filepath.Join(c.Dir, "worktrees", "invalid-field-wt")

	_, stderr, code = c.RunWithInput(nil, "--config", "../config.json", "-C", wtPath, "info", "--field", "nonexistent")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "invalid field")
}

func Test_Info_Appears_In_Help_Output(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	AssertContains(t, stdout, "info")
}

func Test_Info_Works_From_Subdirectory(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "subdir-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Create a subdirectory inside the worktree
	wtPath := filepath.Join(c.Dir, "worktrees", "subdir-test-wt")
	subDir := filepath.Join(wtPath, "src", "nested")

	err := os.MkdirAll(subDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Run info from inside the subdirectory
	stdout, stderr, code := c.RunWithInput(nil, "--config", "../../../../config.json", "-C", subDir, "info")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        subdir-test-wt")
	AssertContains(t, stdout, "base_branch: master")
}
