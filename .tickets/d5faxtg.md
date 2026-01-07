---
schema_version: 1
id: d5faxtg
status: closed
closed: 2026-01-07T21:30:35Z
blocked-by: [d5fattr]
created: 2026-01-07T19:07:22Z
type: task
priority: 2
---
# Support project-level config (.wt/config.json)

## Overview
Implement project-level configuration loading from .wt/config.json in the repository root.

## Background & Rationale
Per SPEC.md, config precedence is:
1. --config flag (explicit path)
2. Project config: .wt/config.json in repository root
3. User config: ~/.config/wt/config.json (or $XDG_CONFIG_HOME/wt/config.json)
4. Built-in defaults

Project and user configs should be MERGED (not replaced), with project config taking precedence.

## Current State
LoadConfig only looks at:
- --config flag path
- ~/.config/wt/config.json
- Defaults

Missing: .wt/config.json in repo root

## Implementation Details

### Updated LoadConfigInput
```go
// LoadConfigInput holds the inputs for LoadConfig.
type LoadConfigInput struct {
    WorkDirOverride string            // -C/--cwd flag value
    ConfigPath      string            // -c/--config flag value
    Env             map[string]string // Environment variables (from Run())
}
```

### Updated LoadConfig
```go
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
    
    // If explicit config path, use only that
    if input.ConfigPath != "" {
        cfg, err := loadConfigFile(fsys, input.ConfigPath)
        if err != nil {
            return Config{}, err
        }
        cfg.EffectiveCwd = workDir
        return applyDefaults(cfg), nil
    }
    
    // Start with defaults
    cfg := DefaultConfig()
    
    // Load user config (lowest precedence)
    userConfigPath := getUserConfigPath(input.Env)
    if userConfigPath != "" {
        if userCfg, err := loadConfigFile(fsys, userConfigPath); err == nil {
            cfg = mergeConfigs(cfg, userCfg)
        }
        // Ignore errors (file might not exist)
    }
    
    // Load project config (higher precedence)
    repoRoot, err := gitRepoRoot(workDir)
    if err == nil {
        projectConfigPath := filepath.Join(repoRoot, ".wt", "config.json")
        if projectCfg, err := loadConfigFile(fsys, projectConfigPath); err == nil {
            cfg = mergeConfigs(cfg, projectCfg)
        }
    }
    
    cfg.EffectiveCwd = workDir
    return cfg, nil
}

func loadConfigFile(fsys fs.FS, path string) (Config, error) {
    data, err := fsys.ReadFile(path)
    if err != nil {
        return Config{}, err
    }
    
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
    }
    
    return cfg, nil
}

func mergeConfigs(base, override Config) Config {
    result := base
    if override.Base != "" {
        result.Base = override.Base
    }
    // Add more fields as config grows
    return result
}

// getUserConfigPath returns the user config path.
// Uses env map instead of os.Getenv() per TECH_SPEC.
func getUserConfigPath(env map[string]string) string {
    // Check XDG_CONFIG_HOME first (from env map, not os.Getenv)
    if xdg, ok := env["XDG_CONFIG_HOME"]; ok && xdg != "" {
        return filepath.Join(xdg, "wt", "config.json")
    }
    
    // Fall back to ~/.config/wt/config.json
    // Note: os.UserHomeDir() is allowed - it's not env access
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".config", "wt", "config.json")
}

func applyDefaults(cfg Config) Config {
    if cfg.Base == "" {
        cfg.Base = DefaultConfig().Base
    }
    return cfg
}
```

### Update Run() to Pass Env
```go
// In Run(), update the LoadConfig call:
cfg, err := LoadConfig(fsys, LoadConfigInput{
    WorkDirOverride: *flagCwd,
    ConfigPath:      *flagConfig,
    Env:             env,  // Pass the env map from Run()'s parameter
})
```

## TECH_SPEC Compliance
- Does NOT use `os.Getenv()` - uses env map passed from Run()
- `os.UserHomeDir()` is allowed (it's not environment variable access)
- `os.Getwd()` is allowed (it's cwd, not env)

## Merge Behavior
Currently only `base` field exists. For future fields:
- Empty/zero values in override: keep base value
- Non-empty values in override: use override

## Precedence Example
```
User config:     {"base": "~/worktrees"}
Project config:  {"base": "./local-wt"}
Result:          {"base": "./local-wt"}  (project wins)
```

## Acceptance Criteria
- --config flag takes highest precedence (exclusive)
- Project config (.wt/config.json) loaded if exists
- User config (~/.config/wt/config.json) loaded if exists
- XDG_CONFIG_HOME respected (via env map, not os.Getenv)
- Configs are merged (project > user > defaults)
- Missing configs are not errors
- Invalid JSON in config IS an error

## Testing
- Test with only user config
- Test with only project config
- Test with both (verify merge)
- Test with --config (ignores others)
- Test with invalid JSON (error)
- Test XDG_CONFIG_HOME override (via env map in test)
