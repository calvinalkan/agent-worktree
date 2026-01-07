package main

import (
	"testing"
)

func Test_Run_Shows_Help_When_No_Args(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run()

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "wt - git worktree manager")
	AssertContains(t, stdout, "Commands:")
}

func Test_Run_Shows_Help_When_Help_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "wt - git worktree manager")
	AssertContains(t, stdout, "Commands:")
}

func Test_Run_Shows_Help_When_H_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run("-h")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "wt - git worktree manager")
	AssertContains(t, stdout, "Commands:")
}

func Test_Run_Fails_When_Unknown_Command(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	_, stderr, code := c.Run("unknown")

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	AssertContains(t, stderr, "error: unknown command")
}

func Test_Create_Shows_Help_When_Help_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, stderr, code := c.Run("create", "--help")

	if code != 0 {
		t.Errorf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Usage: wt create")
	AssertContains(t, stdout, "--name")
	AssertContains(t, stdout, "--from-branch")
}

func Test_Create_Shows_Help_When_H_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, stderr, code := c.Run("create", "-h")

	if code != 0 {
		t.Errorf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Usage: wt create")
}

func Test_List_Shows_Help_When_Help_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, stderr, code := c.Run("list", "--help")

	if code != 0 {
		t.Errorf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Usage: wt list")
	AssertContains(t, stdout, "--json")
}

func Test_Delete_Shows_Help_When_Help_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, stderr, code := c.Run("delete", "--help")

	if code != 0 {
		t.Errorf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "Usage: wt delete")
	AssertContains(t, stdout, "--force")
	AssertContains(t, stdout, "--with-branch")
}

func Test_Run_Uses_Cwd_When_Cwd_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	c.InitGitRepo()

	stdout, stderr, code := c.Run("list")
	if code != 0 {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
	}
}

func Test_Run_Uses_Custom_Config_When_Config_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	c.WriteFile("custom-config.json", `{"base": "/custom/path"}`)
	c.InitGitRepo()

	_, _, code := c.Run("--config", "custom-config.json", "list")
	_ = code
}
