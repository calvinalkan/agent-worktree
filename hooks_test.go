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

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\necho 'hook executed'"), 0o755)
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

	if !strings.Contains(stdout.String(), "hook executed") {
		t.Errorf("expected stdout to contain 'hook executed', got: %q", stdout.String())
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

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\nexit 1"), 0o755)
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
echo "SOURCE=$WT_SOURCE"
`

	err = os.WriteFile(hookPath, []byte(hookScript), 0o755)
	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	wtEnv := map[string]string{
		"WT_ID":          "42",
		"WT_AGENT_ID":    "swift-fox",
		"WT_NAME":        "my-worktree",
		"WT_PATH":        "/path/to/worktree",
		"WT_BASE_BRANCH": "main",
		"WT_REPO_ROOT":   "/path/to/repo",
		"WT_SOURCE":      "/path/to/source",
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
		"BASE_BRANCH=main",
		"REPO_ROOT=/path/to/repo",
		"SOURCE=/path/to/source",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got: %q", expected, output)
		}
	}
}

func Test_runHook_Uses_Cwd(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == windowsOS {
		t.Skip("skipping shell script test on Windows")
	}

	repoDir := t.TempDir()
	cwdDir := t.TempDir()
	fsys := fs.NewReal()

	// Create hook that prints pwd
	hookDir := filepath.Join(repoDir, ".wt", "hooks")

	err := os.MkdirAll(hookDir, 0o750)
	if err != nil {
		t.Fatalf("failed to create hook dir: %v", err)
	}

	hookPath := filepath.Join(hookDir, "post-create")

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\npwd"), 0o755)
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
		cwdDir, // Different from repoDir
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Normalize paths for comparison (handle symlinks)
	expectedCwd, _ := filepath.EvalSymlinks(cwdDir)
	actualCwd := strings.TrimSpace(stdout.String())
	actualCwd, _ = filepath.EvalSymlinks(actualCwd)

	if actualCwd != expectedCwd {
		t.Errorf("expected cwd %q, got %q", expectedCwd, actualCwd)
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

	env := hookEnv(info, "/path/to/wt", "/path/to/repo", "/source/dir")

	expected := map[string]string{
		"WT_ID":          "123",
		"WT_AGENT_ID":    "brave-owl",
		"WT_NAME":        "my-feature",
		"WT_PATH":        "/path/to/wt",
		"WT_BASE_BRANCH": "develop",
		"WT_REPO_ROOT":   "/path/to/repo",
		"WT_SOURCE":      "/source/dir",
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

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\necho post-create-ran"), 0o755)
	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	runner := NewHookRunner(fsys, dir, map[string]string{"PATH": os.Getenv("PATH")}, &stdout, &stderr)

	info := &WorktreeInfo{Name: "test", AgentID: "test-id", ID: 1, BaseBranch: "main"}

	err = runner.RunPostCreate(context.Background(), info, "/wt/path", dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout.String(), "post-create-ran") {
		t.Errorf("expected stdout to contain 'post-create-ran', got: %q", stdout.String())
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

	err = os.WriteFile(hookPath, []byte("#!/bin/bash\necho pre-delete-ran"), 0o755)
	if err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}

	var stdout, stderr bytes.Buffer

	runner := NewHookRunner(fsys, dir, map[string]string{"PATH": os.Getenv("PATH")}, &stdout, &stderr)

	info := &WorktreeInfo{Name: "test", AgentID: "test-id", ID: 1, BaseBranch: "main"}

	err = runner.RunPreDelete(context.Background(), info, "/wt/path", dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout.String(), "pre-delete-ran") {
		t.Errorf("expected stdout to contain 'pre-delete-ran', got: %q", stdout.String())
	}
}
