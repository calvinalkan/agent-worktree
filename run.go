package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/calvinalkan/agent-task/pkg/fs"
)

// Run is the main entry point. Returns exit code.
// sigCh can be nil if signal handling is not needed (e.g., in tests).
func Run(stdin io.Reader, stdout, stderr io.Writer, args []string, _ map[string]string, sigCh <-chan os.Signal) int {
	// Create fresh global flags for this invocation
	globalFlags := flag.NewFlagSet("wt", flag.ContinueOnError)
	globalFlags.SetInterspersed(false)
	globalFlags.Usage = func() {}
	globalFlags.SetOutput(&strings.Builder{})

	flagHelp := globalFlags.BoolP("help", "h", false, "Show help")
	flagCwd := globalFlags.StringP("cwd", "C", "", "Run as if started in `dir`")
	flagConfig := globalFlags.StringP("config", "c", "", "Use specified config `file`")

	err := globalFlags.Parse(args[1:])
	if err != nil {
		fprintln(stderr, "error:", err)
		printGlobalOptions(stderr)

		return 1
	}

	// Create filesystem abstraction
	fsys := fs.NewReal()

	// Load config (handles --cwd resolution internally)
	cfg, err := LoadConfig(fsys, LoadConfigInput{
		WorkDirOverride: *flagCwd,
		ConfigPath:      *flagConfig,
	})
	if err != nil {
		fprintln(stderr, "error:", err)

		return 1
	}

	// Create all commands
	commands := []*Command{
		CreateCmd(cfg, fsys),
		ListCmd(cfg, fsys),
		DeleteCmd(cfg, fsys),
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
	WorkDirOverride string // -C/--cwd flag value; if empty, os.Getwd() is used
	ConfigPath      string // -c/--config flag value
}

// LoadConfig loads configuration from file or returns defaults.
func LoadConfig(fsys fs.FS, input LoadConfigInput) (Config, error) {
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

	configPath := input.ConfigPath
	if configPath == "" {
		// Use default location - if home dir unavailable, use defaults
		configPath = defaultConfigPath()

		if configPath == "" {
			cfg := DefaultConfig()
			cfg.EffectiveCwd = workDir

			return cfg, nil
		}
	}

	data, err := fsys.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.EffectiveCwd = workDir

			return cfg, nil
		}

		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	// Apply defaults for missing fields
	if cfg.Base == "" {
		cfg.Base = DefaultConfig().Base
	}

	cfg.EffectiveCwd = workDir

	return cfg, nil
}

// defaultConfigPath returns the default config file path.
// Returns empty string if home directory cannot be determined.
func defaultConfigPath() string {
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
