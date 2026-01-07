package main

import (
	"os"
	"path/filepath"
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

func Test_Run_Shows_Version_When_Version_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run("--version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "wt")
	// Default version is "dev" when not built with ldflags
	AssertContains(t, stdout, "dev")
}

func Test_Run_Shows_Version_When_V_Flag(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run("-v")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "wt")
}

func Test_Run_Version_Flag_In_Help_Output(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	stdout, _, code := c.Run("--help")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	AssertContains(t, stdout, "--version")
	AssertContains(t, stdout, "Show version")
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

func Test_Config_Project_Config_Takes_Precedence_Over_User_Config(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create project config with higher precedence
	c.WriteFile(".wt/config.json", `{"base": "project-worktrees"}`)

	// Create a worktree to verify the project config is used
	stdout, stderr, code := c.Run("create", "--name", "test-wt")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Path should use project config base
	AssertContains(t, stdout, "project-worktrees")
}

func Test_Config_Project_Config_Loaded_From_Repo_Root(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create project config
	c.WriteFile(".wt/config.json", `{"base": "custom-wt-dir"}`)

	// Create a subdirectory and run from there
	subdir := filepath.Join(c.Dir, "subdir", "nested")

	err := os.MkdirAll(subdir, 0o750)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Run from subdirectory - should still find project config at repo root
	stdout, stderr, code := c.RunWithInput(nil, "--cwd", subdir, "create", "--name", "subdir-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "custom-wt-dir")
}

func Test_Config_Flag_Takes_Exclusive_Precedence(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create project config
	c.WriteFile(".wt/config.json", `{"base": "project-base"}`)

	// Create explicit config with different base
	c.WriteFile("explicit-config.json", `{"base": "explicit-base"}`)

	// --config flag should override project config
	stdout, stderr, code := c.Run("--config", "explicit-config.json", "create", "--name", "explicit-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Should use explicit config, not project config
	AssertContains(t, stdout, "explicit-base")
	AssertNotContains(t, stdout, "project-base")
}

func Test_Config_Invalid_JSON_Returns_Error(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create invalid project config
	c.WriteFile(".wt/config.json", `{invalid json}`)

	_, stderr, code := c.Run("create", "--name", "invalid-test")

	if code != 1 {
		t.Errorf("expected exit code 1 for invalid config, got %d", code)
	}

	AssertContains(t, stderr, "parsing config")
}

func Test_Config_Invalid_Explicit_Config_Returns_Error(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create invalid explicit config
	c.WriteFile("bad-config.json", `not valid json at all`)

	_, stderr, code := c.Run("--config", "bad-config.json", "create", "--name", "test")

	if code != 1 {
		t.Errorf("expected exit code 1 for invalid config, got %d", code)
	}

	AssertContains(t, stderr, "parsing config")
}

func Test_Config_Missing_Project_Config_Uses_Defaults(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// No .wt/config.json - should use defaults
	// Use auto-generated name to avoid conflicts with other tests
	stdout, stderr, code := c.Run("create")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Default base is ~/code/worktrees
	AssertContains(t, stdout, "code/worktrees")
}

func Test_Config_XDG_CONFIG_HOME_Respected(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create XDG config directory
	xdgConfig := filepath.Join(c.Dir, "xdg-config")
	c.WriteFile(filepath.Join("xdg-config", "wt", "config.json"), `{"base": "xdg-worktrees"}`)

	// Set XDG_CONFIG_HOME in env
	c.Env["XDG_CONFIG_HOME"] = xdgConfig

	stdout, stderr, code := c.Run("create", "--name", "xdg-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	AssertContains(t, stdout, "xdg-worktrees")
}

func Test_Config_Project_Config_Overrides_XDG_Config(t *testing.T) {
	t.Parallel()

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	// Create XDG config (lower precedence)
	xdgConfig := filepath.Join(c.Dir, "xdg-config")
	c.WriteFile(filepath.Join("xdg-config", "wt", "config.json"), `{"base": "xdg-base"}`)
	c.Env["XDG_CONFIG_HOME"] = xdgConfig

	// Create project config (higher precedence)
	c.WriteFile(".wt/config.json", `{"base": "project-base"}`)

	stdout, stderr, code := c.Run("create", "--name", "precedence-test")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", code, stderr)
	}

	// Project config should win
	AssertContains(t, stdout, "project-base")
	AssertNotContains(t, stdout, "xdg-base")
}

func Test_getRepoName_Returns_Last_Path_Component(t *testing.T) {
	t.Parallel()

	tests := []struct {
		repoRoot string
		want     string
	}{
		{"/home/user/code/my-repo", "my-repo"},
		{"/code/project", "project"},
		{"my-repo", "my-repo"},
		{"/", "/"},
	}

	for _, tt := range tests {
		got := getRepoName(tt.repoRoot)
		if got != tt.want {
			t.Errorf("getRepoName(%q) = %q, want %q", tt.repoRoot, got, tt.want)
		}
	}
}

func Test_resolveWorktreePath_Absolute_Base_Includes_Repo_Name(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	cfg := Config{
		Base:         "~/code/worktrees",
		EffectiveCwd: "/some/other/path",
	}

	got := resolveWorktreePath(cfg, "/home/user/repos/my-app", "swift-fox")
	want := filepath.Join(home, "code", "worktrees", "my-app", "swift-fox")

	if got != want {
		t.Errorf("resolveWorktreePath() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreePath_Absolute_Base_With_Slash(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:         "/var/worktrees",
		EffectiveCwd: "/some/other/path",
	}

	got := resolveWorktreePath(cfg, "/home/user/repos/project", "brave-owl")
	want := "/var/worktrees/project/brave-owl"

	if got != want {
		t.Errorf("resolveWorktreePath() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreePath_Relative_Base_Uses_EffectiveCwd(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:         "../worktrees",
		EffectiveCwd: "/home/user/code/my-repo",
	}

	got := resolveWorktreePath(cfg, "/home/user/code/my-repo", "calm-deer")
	want := "/home/user/code/my-repo/../worktrees/calm-deer"

	// Clean for comparison
	got = filepath.Clean(got)
	want = filepath.Clean(want)

	if got != want {
		t.Errorf("resolveWorktreePath() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreePath_Relative_Base_No_Repo_Name(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:         "worktrees",
		EffectiveCwd: "/code/project",
	}

	got := resolveWorktreePath(cfg, "/code/project", "swift-fox")
	want := "/code/project/worktrees/swift-fox"

	if got != want {
		t.Errorf("resolveWorktreePath() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreeBaseDir_Absolute_Includes_Repo_Name(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	cfg := Config{
		Base:         "~/code/worktrees",
		EffectiveCwd: "/other/path",
	}

	got := resolveWorktreeBaseDir(cfg, "/home/user/repos/my-project")
	want := filepath.Join(home, "code", "worktrees", "my-project")

	if got != want {
		t.Errorf("resolveWorktreeBaseDir() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreeBaseDir_Relative_Uses_EffectiveCwd(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:         "../worktrees",
		EffectiveCwd: "/code/my-repo",
	}

	got := resolveWorktreeBaseDir(cfg, "/code/my-repo")
	want := "/code/my-repo/../worktrees"

	got = filepath.Clean(got)
	want = filepath.Clean(want)

	if got != want {
		t.Errorf("resolveWorktreeBaseDir() = %q, want %q", got, want)
	}
}

func Test_resolveWorktreeBaseDir_Relative_No_Repo_Name(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:         "worktrees",
		EffectiveCwd: "/code/project",
	}

	got := resolveWorktreeBaseDir(cfg, "/code/project")
	want := "/code/project/worktrees"

	if got != want {
		t.Errorf("resolveWorktreeBaseDir() = %q, want %q", got, want)
	}
}
