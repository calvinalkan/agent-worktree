package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

const windowsOS = "windows"

// writeExecutableFile writes an executable script, ensuring it's fully synced
// to disk before returning. This avoids "text file busy" errors on exec.
func writeExecutableFile(t *testing.T, path string, content []byte) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("failed to create executable %s: %v", path, err)
	}

	_, err = f.Write(content)
	if err != nil {
		_ = f.Close()

		t.Fatalf("failed to write executable %s: %v", path, err)
	}

	err = f.Sync()
	if err != nil {
		_ = f.Close()

		t.Fatalf("failed to sync executable %s: %v", path, err)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("failed to close executable %s: %v", path, err)
	}

	// Brief sleep to ensure filesystem has fully released the file.
	// This works around "text file busy" errors on some systems.
	time.Sleep(10 * time.Millisecond)
}

func Test_runHook_Skips_When_Hook_Not_Present(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fsys := fs.NewReal()

	var stdout, stderr bytes.Buffer

	err := runHook(
		context.Background(),
		fsys,
		dir, // repoRoot with no .wt/hooks
		"post-create",
		map[string]string{},
		map[string]string{},
		dir,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Errorf("expected no error when hook doesn't exist, got: %v", err)
	}
}

func Test_runHook_Returns_Error_When_Hook_Not_Executable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping executable permission test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create hook directory and non-executable hook
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\necho hello"), 0o644) // Not executable
	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = runHook(
		context.Background(),
		fsys,
		dir,
		"post-create",
		map[string]string{},
		map[string]string{},
		dir,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("expected error for non-executable hook, got nil")
	}

	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error should mention 'not executable': %v", err)
	}
}

func Test_runHook_Executes_Hook_Successfully(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create executable hook
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	writeExecutableFile(t, hookPath, []byte("#!/bin/bash\necho 'hook executed'"))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = runHook(
		context.Background(),
		fsys,
		dir,
		"post-create",
		map[string]string{"PATH": os.Getenv("PATH")},
		map[string]string{},
		dir,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout.String(), "hook(post-create): hook executed") {
		t.Errorf("expected stdout to contain 'hook(post-create): hook executed', got: %q", stdout.String())
	}
}

func Test_runHook_Returns_Error_When_Hook_Fails(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create hook that exits with error
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	writeExecutableFile(t, hookPath, []byte("#!/bin/bash\nexit 1"))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = runHook(
		context.Background(),
		fsys,
		dir,
		"post-create",
		map[string]string{"PATH": os.Getenv("PATH")},
		map[string]string{},
		dir,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("expected error when hook exits non-zero, got nil")
	}

	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error should mention 'failed': %v", err)
	}
}

func Test_runHook_Sets_Environment_Variables(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create hook that prints env vars
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")
	hookScript := `#!/bin/bash
echo "ID=$WT_ID"
echo "AGENT_ID=$WT_AGENT_ID"
echo "NAME=$WT_NAME"
echo "PATH_VAR=$WT_PATH"
echo "BASE_BRANCH=$WT_BASE_BRANCH"
echo "REPO_ROOT=$WT_REPO_ROOT"
`

	writeExecutableFile(t, hookPath, []byte(hookScript))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	wtEnv := map[string]string{
		"WT_ID":          "42",
		"WT_AGENT_ID":    "swift-fox",
		"WT_NAME":        "my-worktree",
		"WT_PATH":        "/path/to/worktree",
		"WT_BASE_BRANCH": "master",
		"WT_REPO_ROOT":   "/path/to/repo",
	}

	err = runHook(
		context.Background(),
		fsys,
		dir,
		"post-create",
		map[string]string{"PATH": os.Getenv("PATH")},
		wtEnv,
		dir,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := stdout.String()

	expectedVars := []string{
		"ID=42",
		"AGENT_ID=swift-fox",
		"NAME=my-worktree",
		"PATH_VAR=/path/to/worktree",
		"BASE_BRANCH=master",
		"REPO_ROOT=/path/to/repo",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got: %q", expected, output)
		}
	}
}

func Test_runHook_Uses_WtPath_As_Working_Directory(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	repoDir := t.TempDir()
	wtPath := t.TempDir() // Simulated worktree path
	fsys := fs.NewReal()

	// Create hook that prints pwd
	hookDir := filepath.Join(repoDir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	writeExecutableFile(t, hookPath, []byte("#!/bin/bash\npwd"))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = runHook(
		context.Background(),
		fsys,
		repoDir,
		"post-create",
		map[string]string{"PATH": os.Getenv("PATH")},
		map[string]string{},
		wtPath, // Hook runs in worktree directory
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Normalize paths for comparison (handle symlinks)
	expectedWtPath, _ := filepath.EvalSymlinks(wtPath)

	// Strip prefix "hook(post-create): " from output
	output := strings.TrimSpace(stdout.String())
	actualPwd := strings.TrimPrefix(output, "hook(post-create): ")
	actualPwd, _ = filepath.EvalSymlinks(actualPwd)

	if actualPwd != expectedWtPath {
		t.Errorf("hook pwd = %q, want %q (wtPath)", actualPwd, expectedWtPath)
	}
}

func Test_hookEnv_Creates_All_Variables(t *testing.T) {
	t.Parallel()

	info := &WorktreeInfo{
		Name:       "my-feature",
		AgentID:    "brave-owl",
		ID:         123,
		BaseBranch: "develop",
		Created:    time.Now(),
	}

	env := hookEnv(info, "/path/to/wt", "/path/to/repo")

	expected := map[string]string{
		"WT_ID":          "123",
		"WT_AGENT_ID":    "brave-owl",
		"WT_NAME":        "my-feature",
		"WT_PATH":        "/path/to/wt",
		"WT_BASE_BRANCH": "develop",
		"WT_REPO_ROOT":   "/path/to/repo",
	}

	for key, want := range expected {
		got, ok := env[key]
		if !ok {
			t.Errorf("missing env var %s", key)

			continue
		}

		if got != want {
			t.Errorf("env[%s] = %q, want %q", key, got, want)
		}
	}

	if len(env) != len(expected) {
		t.Errorf("expected %d env vars, got %d", len(expected), len(env))
	}
}

func Test_HookRunner_RunPostCreate_Calls_Hook(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create post-create hook
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	writeExecutableFile(t, hookPath, []byte("#!/bin/bash\necho post-create-ran"))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	runner := NewHookRunner(fsys, dir, map[string]string{"PATH": os.Getenv("PATH")}, &stdout, &stderr)

	info := &WorktreeInfo{Name: "test", AgentID: "test-id", ID: 1, BaseBranch: "master"}

	err = runner.RunPostCreate(context.Background(), info, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout.String(), "hook(post-create): post-create-ran") {
		t.Errorf("expected stdout to contain 'hook(post-create): post-create-ran', got: %q", stdout.String())
	}
}

func Test_HookRunner_RunPreDelete_Calls_Hook(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	dir := t.TempDir()
	fsys := fs.NewReal()

	// Create pre-delete hook
	hookDir := filepath.Join(dir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "pre-delete")

	writeExecutableFile(t, hookPath, []byte("#!/bin/bash\necho pre-delete-ran"))

	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	runner := NewHookRunner(fsys, dir, map[string]string{"PATH": os.Getenv("PATH")}, &stdout, &stderr)

	info := &WorktreeInfo{Name: "test", AgentID: "test-id", ID: 1, BaseBranch: "master"}

	err = runner.RunPreDelete(context.Background(), info, dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout.String(), "hook(pre-delete): pre-delete-ran") {
		t.Errorf("expected stdout to contain 'hook(pre-delete): pre-delete-ran', got: %q", stdout.String())
	}
}

// E2E tests for hooks with actual CLI commands

func Test_E2E_PostCreate_Hook_Receives_All_Environment_Variables(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create hook that outputs all WT_* variables to a file
	hookScript := `#!/bin/bash
cat > "$WT_PATH/hook-env.txt" << EOF
WT_ID=$WT_ID
WT_AGENT_ID=$WT_AGENT_ID
WT_NAME=$WT_NAME
WT_PATH=$WT_PATH
WT_BASE_BRANCH=$WT_BASE_BRANCH
WT_REPO_ROOT=$WT_REPO_ROOT
EOF
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Create worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "env-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Read the env file created by hook
	envContent := c.ReadFileAt(wtPath, "hook-env.txt")

	// Verify each variable
	AssertContains(t, envContent, "WT_ID=1")
	AssertContains(t, envContent, "WT_NAME=env-test")
	AssertContains(t, envContent, "WT_BASE_BRANCH=master")
	AssertContains(t, envContent, "WT_PATH="+wtPath)

	// Normalize paths for comparison (handle symlinks like /tmp -> /private/tmp on macOS)
	expectedRepoRoot, _ := filepath.EvalSymlinks(c.Dir)
	AssertContains(t, envContent, "WT_REPO_ROOT="+expectedRepoRoot)

	// WT_AGENT_ID should be set (some adjective-animal combo)
	if !strings.Contains(envContent, "WT_AGENT_ID=") {
		t.Error("WT_AGENT_ID should be set")
	}

	// Verify agent_id is not empty
	for line := range strings.SplitSeq(envContent, "\n") {
		if after, ok := strings.CutPrefix(line, "WT_AGENT_ID="); ok {
			agentID := after
			if agentID == "" {
				t.Error("WT_AGENT_ID should not be empty")
			}

			if !strings.Contains(agentID, "-") {
				t.Errorf("WT_AGENT_ID should be in adjective-animal format, got %q", agentID)
			}
		}
	}
}

func Test_E2E_PreDelete_Hook_Receives_All_Environment_Variables(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Create pre-delete hook that writes env to repo root (since worktree will be deleted)
	hookScript := `#!/bin/bash
cat > "$WT_REPO_ROOT/pre-delete-env.txt" << EOF
WT_ID=$WT_ID
WT_AGENT_ID=$WT_AGENT_ID
WT_NAME=$WT_NAME
WT_PATH=$WT_PATH
WT_BASE_BRANCH=$WT_BASE_BRANCH
WT_REPO_ROOT=$WT_REPO_ROOT
EOF
`
	c.WriteExecutable(".wt/hooks/pre-delete", hookScript)

	// Create worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "delete-env-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Remove worktree (with --force because new worktrees have uncommitted .wt/worktree.json)
	_, stderr, code = c.Run("--config", "config.json", "remove", "delete-env-test", "--with-branch", "--force")
	if code != 0 {
		t.Fatalf("remove failed: %s", stderr)
	}

	// Check env was captured before deletion
	envContent := c.ReadFile("pre-delete-env.txt")

	AssertContains(t, envContent, "WT_ID=1")
	AssertContains(t, envContent, "WT_NAME=delete-env-test")
	AssertContains(t, envContent, "WT_BASE_BRANCH=master")

	// WT_AGENT_ID should be set
	if !strings.Contains(envContent, "WT_AGENT_ID=") {
		t.Error("WT_AGENT_ID should be set")
	}
}

func Test_E2E_Hook_Working_Directory_Is_WT_PATH(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Hook that records pwd
	hookScript := `#!/bin/bash
pwd > "$WT_PATH/hook-pwd.txt"
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Create worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "pwd-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)
	hookPwd := strings.TrimSpace(c.ReadFileAt(wtPath, "hook-pwd.txt"))

	// Normalize paths for comparison - hook should run in worktree directory
	expectedWtPath, _ := filepath.EvalSymlinks(wtPath)
	actualPwd, _ := filepath.EvalSymlinks(hookPwd)

	if actualPwd != expectedWtPath {
		t.Errorf("hook pwd = %s, want %s (WT_PATH)", actualPwd, expectedWtPath)
	}
}

func Test_E2E_Hook_Can_Access_Worktree_Path(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Hook that creates a file in the worktree
	hookScript := `#!/bin/bash
echo "hook was here" > "$WT_PATH/created-by-hook.txt"
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Create worktree
	stdout, stderr, code := c.Run("--config", "config.json", "create", "--name", "access-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	wtPath := extractPath(stdout)

	// Verify hook created the file
	if !c.FileExistsAt(wtPath, "created-by-hook.txt") {
		t.Error("hook should have created file in WT_PATH")
	}

	content := c.ReadFileAt(wtPath, "created-by-hook.txt")
	AssertContains(t, content, "hook was here")
}

func Test_E2E_Hook_Can_Access_Repo_Root(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Hook that creates a file in the repo root
	hookScript := `#!/bin/bash
echo "hook was here" > "$WT_REPO_ROOT/hook-marker.txt"
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Create worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "repo-access-test")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Verify hook created the file in repo root
	if !c.FileExists("hook-marker.txt") {
		t.Error("hook should have created file in WT_REPO_ROOT")
	}

	content := c.ReadFile("hook-marker.txt")
	AssertContains(t, content, "hook was here")
}

func Test_E2E_Hook_Receives_Signal_On_Cancellation(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Hook that:
	// 1. Writes 'started' immediately
	// 2. Traps TERM signal and writes 'signal-received'
	// 3. Sleeps forever (until killed)
	hookScript := `#!/bin/bash
echo "started" > "$WT_REPO_ROOT/hook-started.txt"

cleanup() {
    echo "signal-received" > "$WT_REPO_ROOT/hook-signal.txt"
    exit 0
}

trap cleanup TERM INT

# Sleep in a loop so trap can be processed
while true; do
    sleep 0.1
done
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Test timeout - fail fast if something is broken
	testTimeout := 5 * time.Second
	deadline := time.Now().Add(testTimeout)

	// Start create with signal channel
	sigCh := make(chan os.Signal, 1)
	done := c.RunWithSignal(sigCh, "--config", "config.json", "create", "--name", "signal-test")

	// Poll for hook to start
	for !c.FileExists("hook-started.txt") {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for hook to start")
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Send signal
	sigCh <- os.Interrupt

	// Wait for command to finish with timeout
	select {
	case code := <-done:
		// Should exit with 130 (128 + SIGINT)
		if code != 130 {
			t.Errorf("expected exit code 130, got %d", code)
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("timeout waiting for command to finish after signal")
	}

	// Verify signal was received by the hook
	if !c.FileExists("hook-signal.txt") {
		t.Error("hook should have received signal and written hook-signal.txt")
	}

	signalContent := strings.TrimSpace(c.ReadFile("hook-signal.txt"))
	if signalContent != "signal-received" {
		t.Errorf("expected 'signal-received', got %q", signalContent)
	}
}

func Test_E2E_Hook_Is_Killed_If_Ignores_Signal(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// Hook that ignores signals and tries to write after a long sleep.
	// If SIGKILL works, the "survived" file will never be written.
	hookScript := `#!/bin/bash
echo "started" > "$WT_REPO_ROOT/hook-started.txt"

# Ignore all signals
trap '' TERM INT

# Sleep longer than WaitDelay (7s), then write
sleep 20
echo "survived" > "$WT_REPO_ROOT/hook-survived.txt"
`
	c.WriteExecutable(".wt/hooks/post-create", hookScript)

	// Test should complete within 15s (7s WaitDelay + buffer)
	testTimeout := 15 * time.Second
	deadline := time.Now().Add(testTimeout)

	// Start create with signal channel
	sigCh := make(chan os.Signal, 1)
	done := c.RunWithSignal(sigCh, "--config", "config.json", "create", "--name", "stuck-hook-test")

	// Poll for hook to start
	for !c.FileExists("hook-started.txt") {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for hook to start")
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Send signal - hook will ignore it
	sigCh <- os.Interrupt

	// Command should still finish (due to WaitDelay + SIGKILL)
	select {
	case code := <-done:
		// Should exit with 130 (interrupted)
		if code != 130 {
			t.Errorf("expected exit code 130, got %d", code)
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("timeout: hook was not killed after ignoring signal")
	}

	// The hook should have been killed before it could write "survived"
	if c.FileExists("hook-survived.txt") {
		t.Error("hook should have been killed before writing hook-survived.txt")
	}
}

func Test_E2E_PreDelete_Hook_Receives_Signal_On_Cancellation(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// First create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "to-delete")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Hook that traps signals and writes confirmation
	hookScript := `#!/bin/bash
echo "started" > "$WT_REPO_ROOT/hook-started.txt"

cleanup() {
    echo "signal-received" > "$WT_REPO_ROOT/hook-signal.txt"
    exit 0
}

trap cleanup TERM INT

# Sleep in a loop so trap can be processed
while true; do
    sleep 0.1
done
`
	c.WriteExecutable(".wt/hooks/pre-delete", hookScript)

	// Test timeout
	testTimeout := 5 * time.Second
	deadline := time.Now().Add(testTimeout)

	// Start remove with signal channel
	sigCh := make(chan os.Signal, 1)
	done := c.RunWithSignal(sigCh, "--config", "config.json", "remove", "to-delete", "--force")

	// Poll for hook to start
	for !c.FileExists("hook-started.txt") {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for hook to start")
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Send signal
	sigCh <- os.Interrupt

	// Wait for command to finish with timeout
	select {
	case code := <-done:
		if code != 130 {
			t.Errorf("expected exit code 130, got %d", code)
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("timeout waiting for command to finish after signal")
	}

	// Verify signal was received by the hook
	if !c.FileExists("hook-signal.txt") {
		t.Error("hook should have received signal and written hook-signal.txt")
	}

	signalContent := strings.TrimSpace(c.ReadFile("hook-signal.txt"))
	if signalContent != "signal-received" {
		t.Errorf("expected 'signal-received', got %q", signalContent)
	}
}

func Test_E2E_PreDelete_Hook_Is_Killed_If_Ignores_Signal(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	c := NewCLITester(t)
	initRealGitRepo(t, c.Dir)

	c.WriteFile("config.json", `{"base": "worktrees"}`)

	// First create a worktree
	_, stderr, code := c.Run("--config", "config.json", "create", "--name", "to-delete")
	if code != 0 {
		t.Fatalf("create failed: %s", stderr)
	}

	// Hook that ignores signals
	hookScript := `#!/bin/bash
echo "started" > "$WT_REPO_ROOT/hook-started.txt"

# Ignore all signals
trap '' TERM INT

# Sleep longer than WaitDelay (7s), then write
sleep 20
echo "survived" > "$WT_REPO_ROOT/hook-survived.txt"
`
	c.WriteExecutable(".wt/hooks/pre-delete", hookScript)

	// Test should complete within 15s (7s WaitDelay + buffer)
	testTimeout := 15 * time.Second
	deadline := time.Now().Add(testTimeout)

	// Start remove with signal channel
	sigCh := make(chan os.Signal, 1)
	done := c.RunWithSignal(sigCh, "--config", "config.json", "remove", "to-delete", "--force")

	// Poll for hook to start
	for !c.FileExists("hook-started.txt") {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for hook to start")
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Send signal - hook will ignore it
	sigCh <- os.Interrupt

	// Command should still finish (due to WaitDelay + SIGKILL)
	select {
	case code := <-done:
		if code != 130 {
			t.Errorf("expected exit code 130, got %d", code)
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("timeout: hook was not killed after ignoring signal")
	}

	// The hook should have been killed before it could write "survived"
	if c.FileExists("hook-survived.txt") {
		t.Error("hook should have been killed before writing hook-survived.txt")
	}
}
