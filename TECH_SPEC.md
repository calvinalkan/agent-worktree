# wt - Technical Specification

## Overview

This document describes the implementation approach for `wt`, a git worktree manager CLI.

---

## Project Structure

```
wt/
├── main.go         # Entry point, Run(), config, helpers
├── command.go      # Command struct and helpers
├── create.go       # CreateCmd()
├── list.go         # ListCmd()
└── delete.go       # DeleteCmd()
```

Single package (`package main`), flat structure.

---

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/pflag` | POSIX-compliant flag parsing |
| Standard library only otherwise | |

Go version: 1.25+

---

## Filesystem Abstraction

Do not use `os` package functions for filesystem operations directly. These are forbidden by linter rules. Instead, use the `fs.FS` interface abstraction from `github.com/calvinalkan/agent-task/pkg/fs`.

**Forbidden:** `os.Open`, `os.Create`, `os.OpenFile`, `os.ReadFile`, `os.ReadDir`, `os.Mkdir`, `os.MkdirAll`, `os.Stat`, `os.Remove`, `os.RemoveAll`, `os.Rename`, `os.WriteFile`

**Use instead:** Pass an `fs.FS` instance and call methods like `fsys.Open()`, `fsys.ReadFile()`, `fsys.MkdirAll()`, etc.

For the interface definition, always depend on `fs.FS`, never on the concrete `fs.Real` type.

To view the interface documentation:

```sh
go doc github.com/calvinalkan/agent-task/pkg/fs FS
```

---

## Core Components

### main.go

Contains:
- `main()` - entry point, OS abstractions, signal setup, calls `Run()`
- `Run()` - global flag parsing, config loading, command dispatch
- Config loading helpers
- Git command helpers (shell out to `git`)
- Worktree metadata helpers (read/write `.wt/worktree.json`)
- Name generation (word lists, random selection)
- Hook execution

### command.go

Contains:
- `Command` struct
- `Name()`, `HelpLine()`, `PrintHelp()`, `Run()` methods

### create.go, list.go, delete.go

Each contains:
- Command constructor: `CreateCmd(cfg Config) *Command`
- Execution function: `execCreate(...) error`

---

## Patterns (from tk)

### Entry Point (main.go)

```go
func main() {
    // Parse environment
    env := parseEnv(os.Environ())
    
    // Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    
    // Delegate to Run with OS abstractions
    exitCode := Run(os.Stdout, os.Stderr, os.Args, env, sigCh)
    os.Exit(exitCode)
}
```

### Run Function

```go
func Run(stdout, stderr io.Writer, args []string, env map[string]string, sigCh <-chan os.Signal) int {
    // 1. Parse global flags
    globalFlags := flag.NewFlagSet("wt", flag.ContinueOnError)
    globalFlags.SetInterspersed(false)
    flagHelp := globalFlags.BoolP("help", "h", false, "Show help")
    flagCwd := globalFlags.StringP("cwd", "C", "", "Run as if started in dir")
    flagConfig := globalFlags.StringP("config", "c", "", "Use config file")
    
    if err := globalFlags.Parse(args[1:]); err != nil {
        // print error, return 1
    }
    
    // 2. Load config (handles --cwd resolution internally, no os.Chdir)
    cfg, err := loadConfig(LoadConfigInput{
        WorkDirOverride: *flagCwd,
        ConfigPath:      *flagConfig,
    })
    if err != nil {
        // print error, return 1
    }
    // cfg.EffectiveCwd now holds the resolved working directory
    
    // 4. Build commands
    commands := []*Command{
        CreateCmd(cfg),
        ListCmd(cfg),
        DeleteCmd(cfg),
    }
    
    // 5. Handle help / no command
    commandArgs := globalFlags.Args()
    if *flagHelp || len(commandArgs) == 0 {
        printUsage(stdout, commands)
        return 0
    }
    
    // 6. Dispatch to command
    cmdName := commandArgs[0]
    cmd := findCommand(commands, cmdName)
    if cmd == nil {
        // print error, return 1
    }
    
    // 7. Run with context and signal handling
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    done := make(chan int, 1)
    go func() {
        done <- cmd.Run(ctx, stdout, stderr, commandArgs[1:])
    }()
    
    select {
    case exitCode := <-done:
        return exitCode
    case <-sigCh:
        cancel()
        // graceful shutdown with timeout
    }
}
```

### Command Struct

```go
type Command struct {
    Flags *flag.FlagSet
    Usage string  // "create [flags]" - first word is command name
    Short string  // one-line description
    Long  string  // full description (optional)
    Exec  func(ctx context.Context, stdout, stderr io.Writer, args []string) error
}

func (c *Command) Name() string {
    name, _, _ := strings.Cut(c.Usage, " ")
    return name
}

func (c *Command) HelpLine() string {
    return fmt.Sprintf("  %-20s %s", c.Usage, c.Short)
}

func (c *Command) PrintHelp(w io.Writer) {
    fmt.Fprintln(w, "Usage: wt", c.Usage)
    fmt.Fprintln(w)
    desc := c.Long
    if desc == "" {
        desc = c.Short
    }
    fmt.Fprintln(w, desc)
    if c.Flags != nil && c.Flags.HasFlags() {
        fmt.Fprintln(w)
        fmt.Fprintln(w, "Flags:")
        c.Flags.SetOutput(w)
        c.Flags.PrintDefaults()
    }
}

func (c *Command) Run(ctx context.Context, stdout, stderr io.Writer, args []string) int {
    if err := c.Flags.Parse(args); err != nil {
        // handle --help, print error
        return 1
    }
    if err := c.Exec(ctx, stdout, stderr, c.Flags.Args()); err != nil {
        fmt.Fprintln(stderr, "error:", err)
        return 1
    }
    return 0
}
```

### Command Constructor Pattern

```go
func CreateCmd(cfg Config) *Command {
    fs := flag.NewFlagSet("create", flag.ContinueOnError)
    fs.StringP("name", "n", "", "Custom worktree name")
    fs.StringP("from-branch", "b", "", "Create from branch")
    fs.Bool("copy-changes", false, "Copy uncommitted changes")
    
    return &Command{
        Flags: fs,
        Usage: "create [flags]",
        Short: "Create a new worktree",
        Long:  "Create a new worktree with auto-generated name and unique ID.",
        Exec: func(ctx context.Context, stdout, stderr io.Writer, args []string) error {
            return execCreate(ctx, stdout, stderr, cfg, fs)
        },
    }
}

func execCreate(ctx context.Context, stdout, stderr io.Writer, cfg Config, fs *flag.FlagSet) error {
    // Implementation
}
```

---

## Config

### Structure

```go
type Config struct {
    Base string `json:"base"`
    
    // Resolved paths (computed, not serialized)
    EffectiveCwd string `json:"-"` // Absolute working directory (from -C flag or os.Getwd)
}
```

### Loading

1. If `-c` flag provided, use that path
2. Otherwise use `~/.config/wt/config.json`
3. If file doesn't exist, use defaults
4. If file exists but invalid JSON, return error

### Defaults

```go
var defaultConfig = Config{
    Base: "~/code/worktrees",
}
```

---

## Git Operations

All git operations shell out to the `git` CLI:

```go
func gitWorktreeAdd(path, branch, baseBranch string) error {
    cmd := exec.Command("git", "worktree", "add", "-b", branch, path, baseBranch)
    return cmd.Run()
}

func gitWorktreeRemove(path string) error {
    cmd := exec.Command("git", "worktree", "remove", path)
    return cmd.Run()
}

func gitRepoRoot() (string, error) {
    cmd := exec.Command("git", "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    return strings.TrimSpace(string(out)), err
}

func gitCurrentBranch() (string, error) {
    cmd := exec.Command("git", "branch", "--show-current")
    out, err := cmd.Output()
    return strings.TrimSpace(string(out)), err
}

func gitIsDirty(path string) (bool, error) {
    cmd := exec.Command("git", "-C", path, "status", "--porcelain")
    out, err := cmd.Output()
    return len(out) > 0, err
}

// ....
```

---

## Worktree Metadata

### Structure

```go
type WorktreeInfo struct {
    Name       string    `json:"name"`
    AgentID    string    `json:"agent_id"`
    ID         int       `json:"id"`
    BaseBranch string    `json:"base_branch"`
    Created    time.Time `json:"created"`
}
```

### Location

`.wt/worktree.json` inside each worktree.

### Operations

```go
func writeWorktreeInfo(wtPath string, info WorktreeInfo) error
func readWorktreeInfo(wtPath string) (WorktreeInfo, error)
func findWorktrees(baseDir string) ([]WorktreeInfo, error)  // glob for .wt/worktree.json
```

---

## Name Generation

### Word Lists

```go
var adjectives = []string{
    "swift", "brave", "calm", "bold", "keen",
    "warm", "cool", "wise", "fair", "fond",
    "quick", "slow", "bright", "dark", "light",
    "soft", "hard", "pure", "rare", "true",
    ......
}

var animals = []string{
    "fox", "owl", "elk", "bee", "ant",
    "jay", "cod", "eel", "bat", "ram",
    "cat", "dog", "pig", "cow", "hen",
    "rat", "ape", "yak", "koi", "gnu",
    ......
}
```

### Generation

```go
func generateAgentID(existing []string) (string, error) {
    for i := 0; i < 10; i++ {
        adj := adjectives[rand.Intn(len(adjectives))]
        animal := animals[rand.Intn(len(animals))]
        candidate := adj + "-" + animal
        if !contains(existing, candidate) {
            return candidate, nil
        }
    }
    return "", errors.New("failed to generate unique agent_id after 10 attempts")
}
```

---

## Hook Execution

```go
func runHook(hookPath string, env map[string]string) error {
    if _, err := os.Stat(hookPath); os.IsNotExist(err) {
        return nil  // hook doesn't exist, skip
    }
    
    // Check executable
    info, err := os.Stat(hookPath)
    if err != nil {
        return err
    }
    if info.Mode()&0111 == 0 {
        return fmt.Errorf("hook exists but is not executable: %s", hookPath)
    }
    
    cmd := exec.Command(hookPath)
    cmd.Env = os.Environ()
    for k, v := range env {
        cmd.Env = append(cmd.Env, k+"="+v)
    }
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    return cmd.Run()
}
```

### Environment Variables

```go
hookEnv := map[string]string{
    "WT_ID":          strconv.Itoa(info.ID),
    "WT_AGENT_ID":    info.AgentID,
    "WT_NAME":        info.Name,
    "WT_PATH":        wtPath,
    "WT_BASE_BRANCH": info.BaseBranch,
    "WT_REPO_ROOT":   repoRoot,
    "WT_SOURCE":      sourceDir,
}
```

---

## TTY Detection

```go
func isTerminal() bool {
    stat, _ := os.Stdin.Stat()
    return (stat.Mode() & os.ModeCharDevice) != 0
}
```

---

## Error Handling

- All errors returned up the call stack
- Commands print errors and return exit code
- No panics
- Context cancellation respected for graceful shutdown

---

## Testing Approach

- Test via `Run()` function with mock stdin/stdout/stderr
- Pass controlled args and env
- No signal channel needed for tests (pass nil)
- Check exit codes and output
- Always write e2e tests, never write unit tests. Tets should run with the real git binary! in a tmp dir.

