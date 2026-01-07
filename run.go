package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

// Version information. Set via ldflags during build:
//
//	go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123 -X main.date=2025-01-07"
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Run is the main entry point. Returns exit code.
// sigCh can be nil if signal handling is not needed (e.g., in tests).
func Run(stdin io.Reader, stdout, stderr io.Writer, args []string, env map[string]string, sigCh <-chan os.Signal) int {
	// Create fresh global flags for this invocation
	globalFlags := flag.NewFlagSet("wt", flag.ContinueOnError)
	globalFlags.SetInterspersed(false)
	globalFlags.Usage = func() {}
	globalFlags.SetOutput(&strings.Builder{})

	flagHelp := globalFlags.BoolP("help", "h", false, "Show help")
	flagVersion := globalFlags.BoolP("version", "v", false, "Show version and exit")
	flagCwd := globalFlags.StringP("cwd", "C", "", "Run as if started in `dir`")
	flagConfig := globalFlags.StringP("config", "c", "", "Use specified config `file`")

	err := globalFlags.Parse(args[1:])
	if err != nil {
		fprintln(stderr, "error:", err)
		printGlobalOptions(stderr)

		return 1
	}

	// Handle --version early, before loading config
	if *flagVersion {
		fprintf(stdout, "wt %s (%s, %s)\n", version, commit, date)

		return 0
	}

	// Create filesystem abstraction
	fsys := fs.NewReal()

	// Create git with explicit environment for isolation
	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	git := NewGit(envSlice)

	// Load config (handles --cwd resolution internally)
	cfg, err := LoadConfig(fsys, git, LoadConfigInput{
		WorkDirOverride: *flagCwd,
		ConfigPath:      *flagConfig,
		Env:             env,
	})
	if err != nil {
		fprintln(stderr, "error:", err)

		return 1
	}

	// Create all commands
	commands := []*Command{
		CreateCmd(cfg, fsys, git, env),
		ListCmd(cfg, fsys, git),
		InfoCmd(cfg, fsys, git),
		DeleteCmd(cfg, fsys, git, env),
	}

	commandMap := make(map[string]*Command, len(commands))
	for _, cmd := range commands {
		commandMap[cmd.Name()] = cmd
	}

	commandAndArgs := globalFlags.Args()

	// Show help: explicit --help or bare `wt` with no args
	if *flagHelp || len(commandAndArgs) == 0 {
		printUsage(stdout, commands)

		return 0
	}

	// Dispatch to command
	cmdName := commandAndArgs[0]

	cmd, ok := commandMap[cmdName]
	if !ok {
		fprintln(stderr, "error: unknown command:", cmdName)
		printUsage(stderr, commands)

		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run command in goroutine so we can handle signals
	done := make(chan int, 1)

	go func() {
		done <- cmd.Run(ctx, stdin, stdout, stderr, commandAndArgs[1:])
	}()

	// Handle nil sigCh for tests
	if sigCh == nil {
		return <-done
	}

	// Wait for completion or first signal
	select {
	case exitCode := <-done:
		return exitCode
	case <-sigCh:
		fprintln(stderr, "shutting down with 5s timeout...")
		cancel()
	}

	// Wait for completion, timeout, or second signal
	select {
	case <-done:
		fprintln(stderr, "graceful shutdown ok (130)")

		return 130
	case <-time.After(5 * time.Second):
		fprintln(stderr, "graceful shutdown timed out, forced exit (130)")

		return 130
	case <-sigCh:
		fprintln(stderr, "graceful shutdown interrupted, forced exit (130)")

		return 130
	}
}

func fprintln(output io.Writer, a ...any) {
	_, _ = fmt.Fprintln(output, a...)
}

func fprintf(output io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(output, format, a...)
}

const globalOptionsHelp = `  -h, --help             Show help
  -v, --version          Show version and exit
  -C, --cwd <dir>        Run as if started in <dir>
  -c, --config <file>    Use specified config file`

func printGlobalOptions(output io.Writer) {
	fprintln(output, "Usage: wt [flags] <command> [args]")
	fprintln(output)
	fprintln(output, "Global flags:")
	fprintln(output, globalOptionsHelp)
	fprintln(output)
	fprintln(output, "Run 'wt --help' for a list of commands.")
}

func printUsage(output io.Writer, commands []*Command) {
	fprintln(output, "wt - git worktree manager")
	fprintln(output)
	fprintln(output, "Usage: wt [flags] <command> [args]")
	fprintln(output)
	fprintln(output, "Flags:")
	fprintln(output, globalOptionsHelp)
	fprintln(output)
	fprintln(output, "Commands:")

	for _, cmd := range commands {
		fprintln(output, cmd.HelpLine())
	}
}

// Config holds the application configuration.
type Config struct {
	Base string `json:"base"`

	// Resolved paths (computed, not serialized)
	EffectiveCwd string `json:"-"` // Absolute working directory (from -C flag or os.Getwd)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Base: "~/code/worktrees",
	}
}

// LoadConfigInput holds the inputs for LoadConfig.
type LoadConfigInput struct {
	WorkDirOverride string            // -C/--cwd flag value; if empty, os.Getwd() is used
	ConfigPath      string            // -c/--config flag value
	Env             map[string]string // Environment variables (for XDG_CONFIG_HOME)
}

// LoadConfig loads configuration with the following precedence (highest first):
// 1. --config flag (explicit path) - if provided, uses ONLY this file
// 2. Project config: .wt/config.json in repository root
// 3. User config: $XDG_CONFIG_HOME/wt/config.json or ~/.config/wt/config.json
// 4. Built-in defaults
//
// Project and user configs are merged, with project taking precedence.
func LoadConfig(fsys fs.FS, git *Git, input LoadConfigInput) (Config, error) {
	// Resolve effective working directory
	workDir := input.WorkDirOverride
	if workDir == "" {
		var err error

		workDir, err = os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("cannot get working directory: %w", err)
		}
	}

	// Make workDir absolute
	if !filepath.IsAbs(workDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("cannot get working directory: %w", err)
		}

		workDir = filepath.Join(cwd, workDir)
	}

	// If explicit config path provided, use ONLY that file
	if input.ConfigPath != "" {
		configPath := input.ConfigPath
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(workDir, configPath)
		}

		cfg, err := loadConfigFile(fsys, configPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cfg = DefaultConfig()
				cfg.EffectiveCwd = workDir

				return cfg, nil
			}

			return Config{}, err
		}

		cfg = applyConfigDefaults(cfg)
		cfg.EffectiveCwd = workDir

		return cfg, nil
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Load user config (lowest precedence after defaults)
	userConfigPath := getUserConfigPath(input.Env)
	if userConfigPath != "" {
		userCfg, loadErr := loadConfigFile(fsys, userConfigPath)
		if loadErr == nil {
			cfg = mergeConfigs(cfg, userCfg)
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			// File exists but is invalid - this is an error
			return Config{}, loadErr
		}
	}

	// Load project config (higher precedence than user config)
	repoRoot, err := git.RepoRoot(workDir)
	if err == nil {
		projectConfigPath := filepath.Join(repoRoot, ".wt", "config.json")

		projectCfg, loadErr := loadConfigFile(fsys, projectConfigPath)
		if loadErr == nil {
			cfg = mergeConfigs(cfg, projectCfg)
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			// File exists but is invalid - this is an error
			return Config{}, loadErr
		}
	}

	cfg.EffectiveCwd = workDir

	return cfg, nil
}

// loadConfigFile loads and parses a config file.
func loadConfigFile(fsys fs.FS, path string) (Config, error) {
	data, err := fsys.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}

// mergeConfigs merges override into base, with override taking precedence.
// Empty/zero values in override do not override base values.
func mergeConfigs(base, override Config) Config {
	result := base

	if override.Base != "" {
		result.Base = override.Base
	}

	return result
}

// applyConfigDefaults fills in missing fields with default values.
func applyConfigDefaults(cfg Config) Config {
	if cfg.Base == "" {
		cfg.Base = DefaultConfig().Base
	}

	return cfg
}

// getUserConfigPath returns the user config path.
// Uses env map for XDG_CONFIG_HOME instead of os.Getenv().
func getUserConfigPath(env map[string]string) string {
	if xdg, ok := env["XDG_CONFIG_HOME"]; ok && xdg != "" {
		return filepath.Join(xdg, "wt", "config.json")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "wt", "config.json")
}

// ExpandPath expands ~ to home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return filepath.Join(home, path[2:])
	}

	return path
}

// IsAbsolutePath returns true if path is absolute (starts with / or ~).
func IsAbsolutePath(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~")
}

// IsTerminal returns true if stdin is a terminal.
func IsTerminal() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return (stat.Mode() & os.ModeCharDevice) != 0
}

// getRepoName extracts the repository name from the root path.
// Returns the last path component (directory name).
func getRepoName(repoRoot string) string {
	return filepath.Base(repoRoot)
}

// resolveWorktreePath computes the full path for a new worktree.
//
// If base is absolute (starts with / or ~):
//
//	<base>/<repo-name>/<worktree-name>
//
// If base is relative:
//
//	<effective-cwd>/<base>/<worktree-name>
//
// Examples:
//
//	base=~/code/worktrees, repo=myapp, name=swift-fox
//	  => /home/user/code/worktrees/myapp/swift-fox
//
//	base=../worktrees, cwd=/code/myapp, name=swift-fox
//	  => /code/worktrees/swift-fox
func resolveWorktreePath(cfg Config, repoRoot, worktreeName string) string {
	base := ExpandPath(cfg.Base)

	if IsAbsolutePath(cfg.Base) {
		// Absolute: include repo name in path
		repoName := getRepoName(repoRoot)

		return filepath.Join(base, repoName, worktreeName)
	}

	// Relative: resolve from effective cwd, no repo name
	return filepath.Join(cfg.EffectiveCwd, base, worktreeName)
}

// resolveWorktreeBaseDir returns the directory containing worktrees for a repo.
// Used by list/delete to find existing worktrees.
func resolveWorktreeBaseDir(cfg Config, repoRoot string) string {
	base := ExpandPath(cfg.Base)

	if IsAbsolutePath(cfg.Base) {
		repoName := getRepoName(repoRoot)

		return filepath.Join(base, repoName)
	}

	return filepath.Join(cfg.EffectiveCwd, base)
}

// WorktreeInfo holds metadata for a wt-managed worktree.
// Stored in .wt/worktree.json within each worktree.
type WorktreeInfo struct {
	Name       string    `json:"name"`
	AgentID    string    `json:"agent_id"`
	ID         int       `json:"id"`
	BaseBranch string    `json:"base_branch"`
	Created    time.Time `json:"created"`
}

// writeWorktreeInfo writes metadata to .wt/worktree.json in the worktree.
func writeWorktreeInfo(fsys fs.FS, wtPath string, info *WorktreeInfo) error {
	wtDir := filepath.Join(wtPath, ".wt")

	mkdirErr := fsys.MkdirAll(wtDir, 0o750)
	if mkdirErr != nil {
		return fmt.Errorf("creating .wt directory: %w", mkdirErr)
	}

	data, marshalErr := json.MarshalIndent(info, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshaling worktree info: %w", marshalErr)
	}

	infoPath := filepath.Join(wtDir, "worktree.json")

	file, createErr := fsys.Create(infoPath)
	if createErr != nil {
		return fmt.Errorf("creating worktree.json: %w", createErr)
	}

	_, writeErr := file.Write(data)
	if writeErr != nil {
		_ = file.Close()

		return fmt.Errorf("writing worktree.json: %w", writeErr)
	}

	syncErr := file.Sync()
	if syncErr != nil {
		_ = file.Close()

		return fmt.Errorf("syncing worktree.json: %w", syncErr)
	}

	closeErr := file.Close()
	if closeErr != nil {
		return fmt.Errorf("closing worktree.json: %w", closeErr)
	}

	return nil
}

// readWorktreeInfo reads metadata from .wt/worktree.json in the worktree.
// Returns os.ErrNotExist if the file doesn't exist.
func readWorktreeInfo(fsys fs.FS, wtPath string) (WorktreeInfo, error) {
	infoPath := filepath.Join(wtPath, ".wt", "worktree.json")

	data, readErr := fsys.ReadFile(infoPath)
	if readErr != nil {
		return WorktreeInfo{}, fmt.Errorf("reading worktree.json: %w", readErr)
	}

	var info WorktreeInfo

	unmarshalErr := json.Unmarshal(data, &info)
	if unmarshalErr != nil {
		return WorktreeInfo{}, fmt.Errorf("parsing worktree.json: %w", unmarshalErr)
	}

	return info, nil
}

// findWorktrees scans the given directory for wt-managed worktrees.
// searchDir should be the directory containing worktree subdirectories.
// Returns worktrees that have .wt/worktree.json files.
func findWorktrees(fsys fs.FS, searchDir string) ([]WorktreeInfo, error) {
	entries, readDirErr := fsys.ReadDir(searchDir)
	if readDirErr != nil {
		if errors.Is(readDirErr, os.ErrNotExist) {
			return nil, nil // No worktrees yet
		}

		return nil, fmt.Errorf("reading worktree directory: %w", readDirErr)
	}

	worktrees := make([]WorktreeInfo, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wtPath := filepath.Join(searchDir, entry.Name())

		info, readInfoErr := readWorktreeInfo(fsys, wtPath)
		if readInfoErr != nil {
			continue // Skip non-wt directories
		}

		worktrees = append(worktrees, info)
	}

	return worktrees, nil
}
