package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/calvinalkan/agent-task/pkg/fs"
	flag "github.com/spf13/pflag"
)

// ListCmd returns the list command.
func ListCmd(cfg Config, fsys fs.FS, git *Git) *Command {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.BoolP("help", "h", false, "Show help")
	flags.Bool("json", false, "Output as JSON")

	return &Command{
		Flags: flags,
		Usage: "list [flags]",
		Short: "List worktrees for current repo",
		Long:  `List all worktrees managed by wt for the current repository.`,
		Exec: func(_ context.Context, stdin io.Reader, stdout, stderr io.Writer, _ []string) error {
			return execList(stdin, stdout, stderr, cfg, fsys, git, flags)
		},
	}
}

func execList(_ io.Reader, stdout, _ io.Writer, cfg Config, fsys fs.FS, git *Git, flags *flag.FlagSet) error {
	jsonOutput, _ := flags.GetBool("json")

	// Get main repo root (works from inside worktrees too)
	mainRepoRoot, err := git.MainRepoRoot(cfg.EffectiveCwd)
	if err != nil {
		return ErrNotGitRepository
	}

	// Find worktrees
	baseDir := resolveWorktreeBaseDir(cfg, mainRepoRoot)

	worktrees, err := findWorktreesWithPaths(fsys, baseDir)
	if err != nil {
		return fmt.Errorf("scanning worktrees: %w", err)
	}

	// Output
	if jsonOutput {
		return outputListJSON(stdout, worktrees)
	}

	return outputListTable(stdout, worktrees)
}

// WorktreeWithPath combines WorktreeInfo with its filesystem path.
type WorktreeWithPath struct {
	WorktreeInfo

	Path string `json:"path"`
}

// findWorktreesWithPaths scans baseDir for wt-managed worktrees and returns them with paths.
func findWorktreesWithPaths(fsys fs.FS, baseDir string) ([]WorktreeWithPath, error) {
	entries, err := fsys.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading directory: %w", err)
	}

	result := make([]WorktreeWithPath, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wtPath := filepath.Join(baseDir, entry.Name())

		info, readErr := readWorktreeInfo(fsys, wtPath)
		if readErr != nil {
			// Not a wt-managed worktree, skip
			continue
		}

		result = append(result, WorktreeWithPath{
			WorktreeInfo: info,
			Path:         wtPath,
		})
	}

	return result, nil
}

func outputListTable(output io.Writer, worktrees []WorktreeWithPath) error {
	if len(worktrees) == 0 {
		return nil // Empty output for no worktrees
	}

	// Header
	fprintf(output, "%-15s %-50s %s\n", "NAME", "PATH", "CREATED")

	for _, wt := range worktrees {
		age := formatAge(wt.Created)
		fprintf(output, "%-15s %-50s %s\n", wt.Name, wt.Path, age)
	}

	return nil
}

func formatAge(t time.Time) string {
	elapsed := time.Since(t)

	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		mins := int(elapsed.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}

		return fmt.Sprintf("%d minutes ago", mins)
	case elapsed < 24*time.Hour:
		hours := int(elapsed.Hours())
		if hours == 1 {
			return "1 hour ago"
		}

		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(elapsed.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}

		return fmt.Sprintf("%d days ago", days)
	}
}

// jsonWorktree is the JSON output format for a worktree.
type jsonWorktree struct {
	Name       string    `json:"name"`
	AgentID    string    `json:"agent_id"`
	ID         int       `json:"id"`
	Path       string    `json:"path"`
	BaseBranch string    `json:"base_branch"`
	Created    time.Time `json:"created"`
}

func outputListJSON(output io.Writer, worktrees []WorktreeWithPath) error {
	result := make([]jsonWorktree, len(worktrees))

	for i, wt := range worktrees {
		result[i] = jsonWorktree{
			Name:       wt.Name,
			AgentID:    wt.AgentID,
			ID:         wt.ID,
			Path:       wt.Path,
			BaseBranch: wt.BaseBranch,
			Created:    wt.Created,
		}
	}

	enc := json.NewEncoder(output)
	enc.SetIndent("", "  ")

	encodeErr := enc.Encode(result)
	if encodeErr != nil {
		return fmt.Errorf("encoding JSON: %w", encodeErr)
	}

	return nil
}
