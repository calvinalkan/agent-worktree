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
