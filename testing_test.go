package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI provides a clean interface for running CLI commands in tests.
// It manages a temp directory and environment variables.
type CLI struct {
	t   *testing.T
	Dir string
	Env map[string]string
}

// NewCLITester creates a new test CLI with a temp directory.
func NewCLITester(t *testing.T) *CLI {
	t.Helper()

	return &CLI{
		t:   t,
		Dir: t.TempDir(),
		Env: map[string]string{},
	}
}

// NewCLITesterAt creates a CLI tester that runs from a specific directory.
// Useful for testing commands that need to run from within a worktree.
func NewCLITesterAt(t *testing.T, dir string) *CLI {
	t.Helper()

	return &CLI{
		t:   t,
		Dir: dir,
		Env: map[string]string{},
	}
}

// Run executes the CLI with the given args and returns stdout, stderr, and exit code.
// Args should not include "wt" or "--cwd" - those are added automatically.
func (c *CLI) Run(args ...string) (string, string, int) {
	return c.RunWithInput(nil, args...)
}

// RunWithInput executes the CLI with stdin and args.
// stdin can be nil, an io.Reader, or a []string (joined with newlines).
func (c *CLI) RunWithInput(stdin any, args ...string) (string, string, int) {
	var inReader io.Reader

	switch v := stdin.(type) {
	case nil:
		inReader = nil
	case io.Reader:
		inReader = v
	case []string:
		inReader = strings.NewReader(strings.Join(v, "\n"))
	default:
		panic(fmt.Sprintf("RunWithInput: stdin must be nil, io.Reader, or []string, got %T", stdin))
	}

	var outBuf, errBuf bytes.Buffer

	fullArgs := append([]string{"wt", "--cwd", c.Dir}, args...)
	code := Run(inReader, &outBuf, &errBuf, fullArgs, c.Env, nil)

	return outBuf.String(), errBuf.String(), code
}

// MustRun executes the CLI and fails the test if the command returns non-zero.
// Returns trimmed stdout on success.
func (c *CLI) MustRun(args ...string) string {
	c.t.Helper()

	stdout, stderr, code := c.Run(args...)
	if code != 0 {
		c.t.Fatalf("command %v failed with exit code %d\nstderr: %s", args, code, stderr)
	}

	return strings.TrimSpace(stdout)
}

// MustFail executes the CLI and fails the test if the command succeeds.
// Returns trimmed stderr.
func (c *CLI) MustFail(args ...string) string {
	c.t.Helper()

	stdout, stderr, code := c.Run(args...)
	if code == 0 {
		c.t.Fatalf("command %v should have failed but succeeded\nstdout: %s", args, stdout)
	}

	return strings.TrimSpace(stderr)
}

// InitGitRepo initializes a git repository in the test directory.
func (c *CLI) InitGitRepo() {
	c.t.Helper()

	// Create minimal git repo
	gitDir := filepath.Join(c.Dir, ".git")

	err := os.MkdirAll(gitDir, 0o750)
	if err != nil {
		c.t.Fatalf("failed to create .git dir: %v", err)
	}

	// Write minimal HEAD file
	headPath := filepath.Join(gitDir, "HEAD")

	err = os.WriteFile(headPath, []byte("ref: refs/heads/master\n"), 0o644)
	if err != nil {
		c.t.Fatalf("failed to write HEAD: %v", err)
	}
}

// WriteFile writes content to a file in the test directory.
func (c *CLI) WriteFile(relPath, content string) {
	c.t.Helper()

	path := filepath.Join(c.Dir, relPath)
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		c.t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		c.t.Fatalf("failed to write file %s: %v", relPath, err)
	}
}

// WriteExecutable writes an executable script to a file in the test directory.
// Used for creating hook scripts that need to be executable.
func (c *CLI) WriteExecutable(relPath, content string) {
	c.t.Helper()

	path := filepath.Join(c.Dir, relPath)
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		c.t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	err = os.WriteFile(path, []byte(content), 0o755)
	if err != nil {
		c.t.Fatalf("failed to write executable %s: %v", relPath, err)
	}
}

// ReadFile reads content from a file in the test directory.
func (c *CLI) ReadFile(relPath string) string {
	c.t.Helper()

	path := filepath.Join(c.Dir, relPath)

	content, err := os.ReadFile(path)
	if err != nil {
		c.t.Fatalf("failed to read file %s: %v", relPath, err)
	}

	return string(content)
}

// FileExists returns true if the file exists in the test directory.
func (c *CLI) FileExists(relPath string) bool {
	path := filepath.Join(c.Dir, relPath)
	_, err := os.Stat(path)

	return err == nil
}

// AssertContains fails the test if content doesn't contain substr.
func AssertContains(t *testing.T, content, substr string) {
	t.Helper()

	if !strings.Contains(content, substr) {
		t.Errorf("content should contain %q\ncontent:\n%s", substr, content)
	}
}

// AssertNotContains fails the test if content contains substr.
func AssertNotContains(t *testing.T, content, substr string) {
	t.Helper()

	if strings.Contains(content, substr) {
		t.Errorf("content should NOT contain %q\ncontent:\n%s", substr, content)
	}
}

// extractPath extracts the path from wt create output.
// Output format is:
//
//	Created worktree:
//	  name:        swift-fox
//	  ...
//	  path:        /path/to/worktree
//	  ...
func extractPath(createOutput string) string {
	return extractField(createOutput, "path")
}

// extractField extracts any field from wt create/info output.
// Returns empty string if field not found.
func extractField(output, field string) string {
	prefix := field + ":"

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(after)
		}
	}

	return ""
}

func Test_ExtractPath_Returns_Path_From_Create_Output(t *testing.T) {
	t.Parallel()

	output := `Created worktree:
  name:        swift-fox
  agent_id:    swift-fox
  id:          1
  path:        /home/user/code/worktrees/my-repo/swift-fox
  branch:      swift-fox
  from:        main`

	got := extractPath(output)

	want := "/home/user/code/worktrees/my-repo/swift-fox"
	if got != want {
		t.Errorf("extractPath() = %q, want %q", got, want)
	}
}

func Test_ExtractPath_Returns_Empty_String_When_Path_Not_Found(t *testing.T) {
	t.Parallel()

	output := `Some other output
without path field`

	got := extractPath(output)
	if got != "" {
		t.Errorf("extractPath() = %q, want empty string", got)
	}
}

func Test_ExtractField_Returns_Name_From_Output(t *testing.T) {
	t.Parallel()

	output := `Created worktree:
  name:        swift-fox
  agent_id:    brave-owl
  id:          42
  path:        /some/path
  branch:      swift-fox
  from:        main`

	got := extractField(output, "name")

	want := "swift-fox"
	if got != want {
		t.Errorf("extractField(name) = %q, want %q", got, want)
	}
}

func Test_ExtractField_Returns_AgentID_From_Output(t *testing.T) {
	t.Parallel()

	output := `Created worktree:
  name:        custom-name
  agent_id:    brave-owl
  id:          42
  path:        /some/path`

	got := extractField(output, "agent_id")

	want := "brave-owl"
	if got != want {
		t.Errorf("extractField(agent_id) = %q, want %q", got, want)
	}
}

func Test_ExtractField_Returns_ID_From_Output(t *testing.T) {
	t.Parallel()

	output := `Created worktree:
  name:        swift-fox
  id:          42
  path:        /some/path`

	got := extractField(output, "id")

	want := "42"
	if got != want {
		t.Errorf("extractField(id) = %q, want %q", got, want)
	}
}

func Test_ExtractField_Returns_Empty_String_When_Field_Not_Found(t *testing.T) {
	t.Parallel()

	output := `Created worktree:
  name:        swift-fox
  id:          1`

	got := extractField(output, "nonexistent")
	if got != "" {
		t.Errorf("extractField(nonexistent) = %q, want empty string", got)
	}
}

func Test_ExtractField_Works_With_Info_Output(t *testing.T) {
	t.Parallel()

	output := `name:        swift-fox
agent_id:    swift-fox
id:          42
path:        /home/user/code/worktrees/my-repo/swift-fox
base_branch: main
created:     2025-01-04T10:30:00Z`

	tests := []struct {
		field string
		want  string
	}{
		{"name", "swift-fox"},
		{"agent_id", "swift-fox"},
		{"id", "42"},
		{"path", "/home/user/code/worktrees/my-repo/swift-fox"},
		{"base_branch", "main"},
		{"created", "2025-01-04T10:30:00Z"},
	}

	for _, tc := range tests {
		got := extractField(output, tc.field)
		if got != tc.want {
			t.Errorf("extractField(%s) = %q, want %q", tc.field, got, tc.want)
		}
	}
}
