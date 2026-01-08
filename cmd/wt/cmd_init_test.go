package main

import (
	"os/exec"
	"strings"
	"testing"
)

func Test_Init_Returns_Error_When_No_Shell_Argument(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	_, stderr, code := c.Run("init")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "missing shell argument")
}

func Test_Init_Returns_Error_When_Unsupported_Shell(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	_, stderr, code := c.Run("init", "fish")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	AssertContains(t, stderr, "unsupported shell")
}

func Test_Init_Bash_Outputs_Valid_Shell_Function(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, stderr, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Verify it's valid bash by having bash parse it
	cmd := exec.Command("bash", "-n", "-c", stdout)

	err := cmd.Run()
	if err != nil {
		t.Errorf("output is not valid bash syntax: %v\noutput:\n%s", err, stdout)
	}

	// Verify it defines a wt function that handles switch
	if !strings.Contains(stdout, "wt()") {
		t.Errorf("output should define wt() function")
	}

	if !strings.Contains(stdout, `"switch"`) {
		t.Errorf("output should handle switch command")
	}
}

func Test_Init_Bash_Switch_Calls_Info_With_Field_Path(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// The switch handler should use wt info with --field path
	if !strings.Contains(stdout, "info") || !strings.Contains(stdout, "--field path") {
		t.Errorf("switch should call 'wt info <id> --field path'\noutput:\n%s", stdout)
	}
}

func Test_Init_Bash_Switch_Shows_Error_For_Missing_Identifier(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Should check for missing identifier
	if !strings.Contains(stdout, "missing worktree identifier") {
		t.Errorf("should show error for missing identifier\noutput:\n%s", stdout)
	}
}

func Test_Init_Bash_Handles_Global_Flags_Before_Switch(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Should handle -C, --cwd, -c, --config flags
	if !strings.Contains(stdout, "-C") || !strings.Contains(stdout, "--config") {
		t.Errorf("should handle global flags\noutput:\n%s", stdout)
	}
}

func Test_Init_Bash_Passes_Through_Other_Commands(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Should pass other commands to wt binary
	if !strings.Contains(stdout, `command wt "$@"`) {
		t.Errorf("should pass through other commands\noutput:\n%s", stdout)
	}
}

func Test_Init_Bash_Handles_Create_With_Switch_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)

	stdout, _, code := c.Run("init", "bash")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Should detect --switch/-s flag
	if !strings.Contains(stdout, "has_switch") {
		t.Errorf("should track --switch flag\noutput:\n%s", stdout)
	}

	// Should handle create with switch specially
	if !strings.Contains(stdout, `"create"`) {
		t.Errorf("should handle create command\noutput:\n%s", stdout)
	}
}
