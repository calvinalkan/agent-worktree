package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func Test_Info_Help_Shows_Detailed_Description(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("info", "--help")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify clarification about wt-managed
	AssertContains(t, stdout, "wt create")

	// Verify scripting examples
	AssertContains(t, stdout, "--field id")
	AssertContains(t, stdout, "--field path")

	// Verify field list in flag description
	AssertContains(t, stdout, "name, agent_id, id, path, base_branch, created")
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

	// Error message should clearly indicate this is a regular branch, not a worktree
	AssertContains(t, stderr, "regular branch")
	AssertContains(t, stderr, "not a worktree")
	AssertContains(t, stderr, "wt list")
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
	AssertContains(t, stderr, "valid: name, agent_id, id, path, base_branch, created")
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

func Test_Info_Field_AgentID_Returns_Value(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "agentid-field-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)
	c2 := NewCLITesterAt(t, wtPath)

	fieldStdout, stderr, code := c2.RunWithInput(nil, "--config", "../config.json", "info", "--field", "agent_id")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// agent_id should be in adjective-animal format
	output := strings.TrimSpace(fieldStdout)
	if !strings.Contains(output, "-") || len(output) < 3 {
		t.Errorf("expected agent_id in adjective-animal format, got %q", output)
	}
}

func Test_Info_Field_Path_Returns_Value(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "path-field-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)
	c2 := NewCLITesterAt(t, wtPath)

	fieldStdout, stderr, code := c2.RunWithInput(nil, "--config", "../config.json", "info", "--field", "path")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// path should contain the worktree name
	output := strings.TrimSpace(fieldStdout)
	if !strings.Contains(output, "path-field-wt") {
		t.Errorf("expected path to contain 'path-field-wt', got %q", output)
	}
}

func Test_Info_Field_Created_Returns_Timestamp(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "created-field-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)
	c2 := NewCLITesterAt(t, wtPath)

	fieldStdout, stderr, code := c2.RunWithInput(nil, "--config", "../config.json", "info", "--field", "created")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// created should be in ISO 8601 format
	output := strings.TrimSpace(fieldStdout)
	if !strings.Contains(output, "T") || !strings.HasSuffix(output, "Z") {
		t.Errorf("expected created in ISO 8601 format (e.g. 2025-01-07T16:30:00Z), got %q", output)
	}
}

func Test_Info_Using_ExtractPath_And_NewCLITesterAt(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree and extract path using helper
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "helpers-test-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Use extractPath helper
	wtPath := extractPath(stdout)
	if wtPath == "" {
		t.Fatal("extractPath returned empty string")
	}

	// Use NewCLITesterAt helper
	c2 := NewCLITesterAt(t, wtPath)

	infoStdout, stderr, code := c2.RunWithInput(nil, "--config", "../config.json", "info")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, infoStdout, "name:        helpers-test-wt")

	// Also verify extractField works with info output
	name := extractField(infoStdout, "name")
	if name != "helpers-test-wt" {
		t.Errorf("extractField(name) = %q, want %q", name, "helpers-test-wt")
	}
}

func Test_Info_Lookup_By_Name(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "lookup-name-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Lookup by name from main repo (not inside worktree)
	stdout, stderr, code := c.Run("--config", "config.json", "info", "lookup-name-wt")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        lookup-name-wt")
	AssertContains(t, stdout, "base_branch: master")
}

func Test_Info_Lookup_By_AgentID(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree (agent_id will be auto-generated)
	stdout, stderr, code := c.Run("--config", "config.json", "create")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Extract the agent_id from create output
	agentID := extractField(stdout, "agent_id")
	if agentID == "" {
		t.Fatal("could not extract agent_id from create output")
	}

	// Lookup by agent_id from main repo
	stdout, stderr, code = c.Run("--config", "config.json", "info", agentID)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "agent_id:    "+agentID)
}

func Test_Info_Lookup_By_NumericID(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "lookup-id-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Lookup by numeric id (first worktree gets id 1)
	stdout, stderr, code := c.Run("--config", "config.json", "info", "1")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        lookup-id-wt")
	AssertContains(t, stdout, "id:          1")
}

func Test_Info_Lookup_Returns_Error_When_Not_Found(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Lookup nonexistent worktree
	_, stderr, code := c.Run("--config", "config.json", "info", "nonexistent-wt")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "worktree not found")
	AssertContains(t, stderr, "nonexistent-wt")
}

func Test_Info_Lookup_With_Field_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "field-lookup-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Lookup by name with --field path
	stdout, stderr, code := c.Run("--config", "config.json", "info", "field-lookup-wt", "--field", "path")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Output should be just the path
	output := strings.TrimSpace(stdout)
	if !strings.Contains(output, "field-lookup-wt") {
		t.Errorf("expected path to contain 'field-lookup-wt', got %q", output)
	}

	// Should not contain other fields
	AssertNotContains(t, stdout, "name:")
	AssertNotContains(t, stdout, "agent_id:")
}

func Test_Info_Lookup_With_JSON_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "json-lookup-wt")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Lookup by name with --json
	stdout, stderr, code := c.Run("--config", "config.json", "info", "json-lookup-wt", "--json")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Parse JSON output
	var info infoJSON

	err := json.Unmarshal([]byte(stdout), &info)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, stdout)
	}

	if info.Name != "json-lookup-wt" {
		t.Errorf("expected name 'json-lookup-wt', got %q", info.Name)
	}
}

func Test_Info_Lookup_Multiple_Worktrees_Finds_Correct_One(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create multiple worktrees
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "wt-alpha")
	if code != 0 {
		t.Fatalf("create wt-alpha failed: %s", stderr)
	}

	_, stderr, code = c.Run("--config", "config.json", "create", "--name", "wt-beta")
	if code != 0 {
		t.Fatalf("create wt-beta failed: %s", stderr)
	}

	_, stderr, code = c.Run("--config", "config.json", "create", "--name", "wt-gamma")
	if code != 0 {
		t.Fatalf("create wt-gamma failed: %s", stderr)
	}

	// Lookup middle one by name
	stdout, stderr, code := c.Run("--config", "config.json", "info", "wt-beta")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        wt-beta")
	AssertContains(t, stdout, "id:          2")

	// Lookup by ID
	stdout, stderr, code = c.Run("--config", "config.json", "info", "3")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "name:        wt-gamma")
}
