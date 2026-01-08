---
schema_version: 1
id: d5fs5rg
status: closed
closed: 2026-01-08T11:31:17Z
blocked-by: []
created: 2026-01-08T11:20:02Z
type: task
priority: 2
---
# Rename delete command to remove (with rm alias)

Rename the 'wt delete' command to 'wt remove' for consistency with git conventions (git worktree remove). Add 'rm' as a short alias.

## Implementation Approach

Add alias support to the Command struct with minimal changes:

1. **Add `Aliases` field to Command struct** (command.go):
   ```go
   type Command struct {
       // ... existing fields ...
       
       // Aliases are alternative names for this command.
       // The primary name comes from Usage, aliases are additional.
       Aliases []string
   }
   ```

2. **Register aliases in commandMap** (cmd_run.go, ~3 lines):
   ```go
   commandMap := make(map[string]*Command, len(commands)*2)
   for _, cmd := range commands {
       commandMap[cmd.Name()] = cmd
       for _, alias := range cmd.Aliases {
           commandMap[alias] = cmd
       }
   }
   ```

3. **Update DeleteCmd to RemoveCmd** (cmd_delete.go → cmd_remove.go):
   - Rename file
   - Change Usage to "remove <name> [flags]"
   - Add `Aliases: []string{"rm"}`

4. **Show aliases in help output**:
   - In global help (`wt --help`), show aliases inline:
     ```
     Commands:
       create [flags]          Create a new worktree
       ls [flags]              List worktrees for current repo
       info [flags]            Show current worktree info
       remove, rm <name>       Remove a worktree
     ```
   - In command help (`wt remove --help`), show aliases section:
     ```
     Usage: wt remove <name> [flags]
     
     Aliases: rm
     
     Remove a worktree...
     ```
   
   Update `HelpLine()` and `PrintHelp()` in command.go to render aliases.

This approach:
- Adds generic alias support usable by any command
- Minimal code change (~10 lines total for the feature)
- No change to command dispatch logic (just adds more map entries)
- Aliases point to same Command instance, so behavior is identical

## Acceptance Criteria

- Command is renamed from 'delete' to 'remove'
- 'rm' works as an alias for 'remove'
- All flags and behavior remain the same
- Global help (`wt --help`) shows aliases inline with command name
- Command help (`wt remove --help`) shows "Aliases: rm" section
- Tests updated to use new command name
- Alias mechanism is generic (can be reused for other commands)
