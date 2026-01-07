---
schema_version: 1
id: d5faxh0
status: open
blocked-by: [d5faw58]
created: 2026-01-07T19:06:44Z
type: task
priority: 2
---
# Implement --with-changes flag for create command

## Overview
Implement the --with-changes flag that copies uncommitted changes to the new worktree.

## Background & Rationale
Per SPEC.md, --with-changes copies "staged, unstaged, and untracked files respecting .gitignore" to the new worktree. This is useful when you want to move work-in-progress to a new branch/worktree without committing.

## Current State
The flag is defined in create.go but the copyUncommittedChanges function is not implemented.

## Implementation Details

### Get Changed Files
```go
// getChangedFiles returns files with uncommitted changes (staged + unstaged).
func getChangedFiles(cwd string) ([]string, error) {
    // Get staged and unstaged changes
    cmd := exec.Command("git", "-C", cwd, "diff", "--name-only", "HEAD")
    out, err := cmd.Output()
    if err != nil {
        // HEAD might not exist (initial commit), try without HEAD
        cmd = exec.Command("git", "-C", cwd, "diff", "--name-only")
        out, _ = cmd.Output()
    }
    
    var files []string
    for _, line := range strings.Split(string(out), "\n") {
        if line = strings.TrimSpace(line); line != "" {
            files = append(files, line)
        }
    }
    
    // Also get staged files (in case some are only staged)
    cmd = exec.Command("git", "-C", cwd, "diff", "--cached", "--name-only")
    out, _ = cmd.Output()
    for _, line := range strings.Split(string(out), "\n") {
        if line = strings.TrimSpace(line); line != "" {
            // Deduplicate
            found := false
            for _, f := range files {
                if f == line {
                    found = true
                    break
                }
            }
            if !found {
                files = append(files, line)
            }
        }
    }
    
    return files, nil
}

// getUntrackedFiles returns untracked files (respecting .gitignore).
func getUntrackedFiles(cwd string) ([]string, error) {
    cmd := exec.Command("git", "-C", cwd, "ls-files", "--others", "--exclude-standard")
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    var files []string
    for _, line := range strings.Split(string(out), "\n") {
        if line = strings.TrimSpace(line); line != "" {
            files = append(files, line)
        }
    }
    return files, nil
}
```

### Copy Files
```go
// copyUncommittedChanges copies staged, unstaged, and untracked files to dst.
func copyUncommittedChanges(fsys fs.FS, srcDir, dstDir string) error {
    changed, err := getChangedFiles(srcDir)
    if err != nil {
        return fmt.Errorf("getting changed files: %w", err)
    }
    
    untracked, err := getUntrackedFiles(srcDir)
    if err != nil {
        return fmt.Errorf("getting untracked files: %w", err)
    }
    
    allFiles := append(changed, untracked...)
    
    for _, relPath := range allFiles {
        srcPath := filepath.Join(srcDir, relPath)
        dstPath := filepath.Join(dstDir, relPath)
        
        // Read source file
        data, err := fsys.ReadFile(srcPath)
        if err != nil {
            // File might have been deleted (shown in diff but gone)
            continue
        }
        
        // Create parent directories
        if err := fsys.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
            return fmt.Errorf("creating directory for %s: %w", relPath, err)
        }
        
        // Write to destination
        f, err := fsys.Create(dstPath)
        if err != nil {
            return fmt.Errorf("creating %s: %w", relPath, err)
        }
        
        if _, err := f.Write(data); err != nil {
            f.Close()
            return fmt.Errorf("writing %s: %w", relPath, err)
        }
        
        if err := f.Sync(); err != nil {
            f.Close()
            return fmt.Errorf("syncing %s: %w", relPath, err)
        }
        
        f.Close()
    }
    
    return nil
}
```

## Edge Cases
- File in diff but deleted: skip silently
- Binary files: copy as-is
- Symlinks: copy target content (or skip?)
- Empty file list: no-op, not an error
- File permissions: preserve? (might need extra handling)

## Acceptance Criteria
- Copies staged files
- Copies unstaged changes
- Copies untracked files
- Respects .gitignore for untracked
- Creates necessary subdirectories
- Handles deleted files gracefully

## Testing
- Create with modified tracked file
- Create with new untracked file
- Create with staged file
- Create with .gitignore'd file (should NOT copy)
- Create with nested directory structure
